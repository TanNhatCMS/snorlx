import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { AddressInfo } from 'node:net';

import { SnorlxApiClient } from './client.js';

interface RecordedRequest {
	method: string;
	path: string;
	authorization: string | null;
}

type Handler = (req: IncomingMessage, res: ServerResponse) => void;

async function startServer(handler: Handler): Promise<{ url: string; requests: RecordedRequest[]; close: () => Promise<void> }> {
	const requests: RecordedRequest[] = [];
	const server = createServer((req, res) => {
		requests.push({
			method: req.method ?? '',
			path: req.url ?? '',
			authorization: req.headers.authorization ?? null,
		});
		handler(req, res);
	});
	await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
	const addr = server.address() as AddressInfo;
	return {
		url: `http://127.0.0.1:${addr.port}`,
		requests,
		close: () => new Promise<void>((resolve) => server.close(() => resolve())),
	};
}

function json(res: ServerResponse, body: unknown): void {
	res.setHeader('Content-Type', 'application/json');
	res.end(JSON.stringify(body));
}

test('getJobLogText resolves a local content URL with the API token', async () => {
	const backend = await startServer((req, res) => {
		if (req.url === '/api/jobs/42/logs') {
			json(res, { url: '/api/jobs/42/logs/content' });
			return;
		}
		if (req.url === '/api/jobs/42/logs/content') {
			res.setHeader('Content-Type', 'text/plain');
			res.end('hello\nworld');
			return;
		}
		res.statusCode = 404;
		res.end();
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getJobLogText(42);

	assert.equal(out.log, 'hello\nworld');
	assert.equal(out.truncated, false);
	assert.equal(out.job_id, 42);
	assert.equal(out.source, 'snorlx');
	// Both hops must carry the Snorlx API token.
	for (const req of backend.requests) {
		assert.equal(req.authorization, 'Bearer tok', `missing auth on ${req.path}`);
	}
	await backend.close();
});

test('fetchLogFromUrl downloads an absolute GitHub signed URL without auth', async () => {
	const github = await startServer((req, res) => {
		assert.equal(req.headers.authorization, undefined, 'signed URLs must not carry the Snorlx token');
		res.setHeader('Content-Type', 'text/plain');
		res.end('github log text');
	});
	const backend = await startServer((req, res) => {
		json(res, { url: `${github.url}/signed/77` });
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.fetchLogFromUrl(`${github.url}/signed/77`);

	assert.equal(out.text, 'github log text');
	assert.equal(out.source, 'github');
	await Promise.all([backend.close(), github.close()]);
});

test('getJobLogText truncates long logs to the last max_chars characters', async () => {
	const content = 'x'.repeat(500) + 'TAIL';
	const backend = await startServer((req, res) => {
		if (req.url === '/api/jobs/1/logs') {
			json(res, { url: '/api/jobs/1/logs/content' });
			return;
		}
		res.setHeader('Content-Type', 'text/plain');
		res.end(content);
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getJobLogText(1, 10);

	assert.equal(out.log.length, 10);
	assert.ok(out.log.endsWith('TAIL'), 'tail truncation must keep the newest content');
	assert.equal(out.truncated, true);
	await backend.close();
});

test('getJobLogText surfaces the backend message when no log URL exists', async () => {
	const backend = await startServer((req, res) => {
		json(res, { message: 'Failed to fetch job logs' });
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	await assert.rejects(() => client.getJobLogText(9), /Failed to fetch job logs/);
	await backend.close();
});

test('getRunLogText aggregates completed job logs and skips queued jobs', async () => {
	const backend = await startServer((req, res) => {
		if (req.url === '/api/runs/7/jobs') {
			json(res, [
				{ id: 1, name: 'build', status: 'completed' },
				{ id: 2, name: 'test', status: 'completed' },
				{ id: 3, name: 'deploy', status: 'queued' },
			]);
			return;
		}
		if (req.url === '/api/jobs/1/logs') {
			json(res, { url: '/api/jobs/1/logs/content' });
			return;
		}
		if (req.url === '/api/jobs/2/logs') {
			json(res, { url: '/api/jobs/2/logs/content' });
			return;
		}
		if (req.url === '/api/jobs/1/logs/content' || req.url === '/api/jobs/2/logs/content') {
			res.setHeader('Content-Type', 'text/plain');
			res.end(`${req.url === '/api/jobs/1/logs/content' ? 'BUILD' : 'TEST'} LOG OUTPUT`);
			return;
		}
		// Any other fetch (e.g. the queued job) must not happen.
		assert.fail(`unexpected request ${req.url}`);
		res.statusCode = 404;
		res.end();
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getRunLogText(7);

	assert.equal(out.run_id, 7);
	assert.ok(out.log.includes('build'), 'aggregated log must contain the build job');
	assert.ok(out.log.includes('BUILD LOG OUTPUT'));
	assert.ok(out.log.includes('test'), 'aggregated log must contain the test job');
	assert.ok(out.log.includes('TEST LOG OUTPUT'));
	assert.equal(out.truncated, false);
	await backend.close();
});

test('getRunLogText truncates the joined log to max_chars', async () => {
	const backend = await startServer((req, res) => {
		if (req.url === '/api/runs/7/jobs') {
			json(res, [
				{ id: 1, name: 'a', status: 'completed' },
				{ id: 2, name: 'b', status: 'completed' },
			]);
			return;
		}
		if (req.url?.endsWith('/logs')) {
			json(res, { url: req.url.replace('/logs', '/logs/content') });
			return;
		}
		res.setHeader('Content-Type', 'text/plain');
		res.end('y'.repeat(1000));
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getRunLogText(7, 50);

	assert.equal(out.truncated, true);
	assert.ok(out.log.length <= 50, `log length ${out.log.length} must fit the cap`);
	await backend.close();
});

test('fetchLogFromUrl never sends the Snorlx token to a protocol-relative foreign URL', async () => {
	const evil = await startServer((req, res) => {
		assert.equal(req.headers.authorization, undefined, 'token must never leave the Snorlx origin');
		res.setHeader('Content-Type', 'text/plain');
		res.end('evil content');
	});
	const backend = await startServer((req, res) => {
		json(res, { url: `${evil.url.replace('http://', '//')}/signed/1` });
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getJobLogText(1);

	assert.equal(out.log, 'evil content');
	assert.equal(out.source, 'github');
	await Promise.all([backend.close(), evil.close()]);
});

test('getRunLogText budgets per job so every completed job keeps its header', async () => {
	const backend = await startServer((req, res) => {
		if (req.url === '/api/runs/7/jobs') {
			json(res, [
				{ id: 1, name: 'alpha', status: 'completed' },
				{ id: 2, name: 'beta', status: 'completed' },
				{ id: 3, name: 'gamma', status: 'completed' },
			]);
			return;
		}
		if (req.url?.endsWith('/logs')) {
			json(res, { url: req.url.replace('/logs', '/logs/content') });
			return;
		}
		res.setHeader('Content-Type', 'text/plain');
		res.end('z'.repeat(2000));
	});
	const client = new SnorlxApiClient(backend.url, 'tok');

	const out = await client.getRunLogText(7, 1500);

	assert.ok(out.log.includes('alpha'), 'first job header must survive');
	assert.ok(out.log.includes('beta'), 'middle job header must survive');
	assert.ok(out.log.includes('gamma'), 'last job header must survive');
	assert.ok(out.log.length <= 1500, `log length ${out.log.length} must fit the cap`);
	assert.ok(out.log.endsWith('zzz'.slice(0, 3)) || out.log.includes('zzz'), 'tails must be kept');
	await backend.close();
});
