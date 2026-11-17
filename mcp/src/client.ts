const DEFAULT_LOG_CHARS = 24_000;

export class SnorlxApiClient {
  constructor(
    private readonly baseUrl: string,
    private readonly token: string,
  ) {}

  async request<T = unknown>(
    method: string,
    path: string,
    options?: { query?: Record<string, string | number | boolean | undefined>; body?: unknown },
  ): Promise<T> {
    const url = new URL(path, this.baseUrl.replace(/\/$/, '') + '/');
    if (options?.query) {
      for (const [key, value] of Object.entries(options.query)) {
        if (value !== undefined && value !== '') {
          url.searchParams.set(key, String(value));
        }
      }
    }

    const headers: Record<string, string> = {
      Accept: 'application/json',
      Authorization: `Bearer ${this.token}`,
    };
    let body: string | undefined;
    if (options?.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }

    const res = await fetch(url, { method, headers, body });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`Snorlx API ${method} ${path} failed (${res.status}): ${text || res.statusText}`);
    }
    if (!text || res.status === 204) {
      return undefined as T;
    }
    try {
      return JSON.parse(text) as T;
    } catch {
      return text as T;
    }
  }

  truncateLogs(payload: unknown, maxChars = DEFAULT_LOG_CHARS): unknown {
    if (typeof payload === 'string') {
      return truncateText(payload, maxChars);
    }
    if (payload && typeof payload === 'object') {
      const obj = { ...(payload as Record<string, unknown>) };
      for (const key of ['logs', 'content', 'log', 'body', 'text']) {
        if (typeof obj[key] === 'string') {
          obj[key] = truncateText(obj[key] as string, maxChars);
          obj.truncated = true;
          obj.truncated_note = `Log truncated to last ${maxChars} characters for MCP context limits.`;
        }
      }
      return obj;
    }
    return payload;
  }

  // ===== Log content =====

  /**
   * Downloads a log URL. Snorlx serves archived logs under a local path that needs the API
   * token; GitHub serves signed absolute URLs that must not carry it.
   */
  async fetchLogFromUrl(url: string): Promise<{ text: string; source: 'snorlx' | 'github' }> {
    const isLocal = url.startsWith('/');
    const headers: Record<string, string> = {};
    if (isLocal) {
      headers.Authorization = `Bearer ${this.token}`;
    }
    const target = isLocal ? new URL(url, this.baseUrl.replace(/\/$/, '') + '/') : new URL(url);
    const res = await fetch(target, { headers });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`Log download from ${isLocal ? url : 'GitHub signed URL'} failed (${res.status}): ${text || res.statusText}`);
    }
    return { text, source: isLocal ? 'snorlx' : 'github' };
  }

  /**
   * Resolves the log URL of one job and downloads its content — archived from Snorlx, live
   * from GitHub when not archived yet.
   */
  async getJobLogText(
    jobId: number,
    maxChars = DEFAULT_LOG_CHARS,
  ): Promise<{ job_id: number; log: string; truncated: boolean; source: string }> {
    const data = await this.request<{ url?: string; message?: string }>('GET', `/api/jobs/${jobId}/logs`);
    if (!data || typeof data !== 'object' || !data.url) {
      const reason = data && typeof data.message === 'string' ? data.message : 'no log URL returned';
      throw new Error(`Job ${jobId} log unavailable: ${reason}`);
    }
    const { text, source } = await this.fetchLogFromUrl(data.url);
    return {
      job_id: jobId,
      log: truncateText(text, maxChars),
      truncated: text.length > maxChars,
      source,
    };
  }

  /**
   * Aggregates the logs of a run's completed jobs into one text block, newest-style headers
   * per job. Queued or in-progress jobs have no log and are skipped.
   */
  async getRunLogText(
    runId: number,
    maxChars = DEFAULT_LOG_CHARS,
  ): Promise<{ run_id: number; log: string; truncated: boolean }> {
    const data = await this.request<unknown>('GET', `/api/runs/${runId}/jobs`);
    const jobs = Array.isArray(data)
      ? (data as Array<{ id: number; name?: string; status?: string }>)
      : ((data as { jobs?: Array<{ id: number; name?: string; status?: string }> } | undefined)?.jobs ?? []);

    const parts: string[] = [];
    let truncated = false;
    for (const job of jobs) {
      if (job.status !== 'completed') {
        continue;
      }
      const jobLog = await this.getJobLogText(job.id, maxChars);
      truncated = truncated || jobLog.truncated;
      parts.push(`### ${job.name ?? 'job'} (job ${job.id})\n${jobLog.log}`);
    }
    const joined = parts.length > 0 ? parts.join('\n\n') : '(no completed jobs with logs for this run)';
    return {
      run_id: runId,
      log: truncateText(joined, maxChars),
      truncated: truncated || joined.length > maxChars,
    };
  }
}

function truncateText(text: string, maxChars: number): string {
  if (text.length <= maxChars) {
    return text;
  }
  return text.slice(text.length - maxChars);
}

export function requireEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}
