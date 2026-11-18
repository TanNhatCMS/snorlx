# Snorlx - CI/CD Dashboard cho GitHub Actions

> **Fork tiếng Việt**: đây là fork [banshee86vr/snorlx](https://github.com/banshee86vr/snorlx) của
> [TanNhatCMS](https://github.com/TanNhatCMS), thêm tính năng **lưu trữ log vĩnh viễn** và **MCP server
> trả nội dung log cho agent**. Tài liệu và giao diện đã dịch sang tiếng Việt; comment trong source
> giữ tiếng Anh để dễ đồng bộ với upstream.

[![CI](https://github.com/banshee86vr/snorlx/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/banshee86vr/snorlx/actions/workflows/ci.yml) [![Security Scans](https://github.com/banshee86vr/snorlx/actions/workflows/security.yml/badge.svg?branch=main&event=push)](https://github.com/banshee86vr/snorlx/actions/workflows/security.yml) [![Release](https://github.com/banshee86vr/snorlx/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/banshee86vr/snorlx/actions/workflows/release.yml) [![Latest release](https://img.shields.io/github/v/release/banshee86vr/snorlx?sort=semver)](https://github.com/banshee86vr/snorlx/releases/latest) [![Go](https://img.shields.io/github/go-mod/go-version/banshee86vr/snorlx?filename=backend%2Fgo.mod)](backend/go.mod) [![License: MIT](https://img.shields.io/github/license/banshee86vr/snorlx)](LICENSE)

Dashboard tự host toàn diện, cho bạn nhìn tập trung mọi pipeline GitHub Actions, số liệu hiệu năng và chi phí phân bố trên nhiều repository và tổ chức.

![Dashboard Preview](docs/preview.png)

## Tính năng

- **Nhìn tập trung**: một màn hình duy nhất cho mọi workflow run trên tất cả repository
- **Chấm điểm repository**: xếp hạng vàng/bạc/đồng theo 7 nhóm: Bảo mật, Kiểm thử, CI/CD, Tài liệu, Chất lượng code, Bảo trì và Cộng đồng
- **Cập nhật realtime**: trạng thái pipeline trực tiếp qua WebSocket, được nuôi bằng webhook và live poller phía server dùng conditional request tới GitHub
- **GitHub OAuth**: đăng nhập bằng GitHub, không cần cấu hình GitHub App
- **Theo dõi chi phí**: phân tích chi phí theo từng workflow và từng repository
- **Hỗ trợ nhiều repository**: theo dõi workflow trên nhiều repo và tổ chức
- **Giao diện đẹp**: thiết kế hiện đại, responsive, có dark/light mode
- **Hai chế độ lưu trữ**: chạy nhanh với memory hoặc bền vững với PostgreSQL

## Kiến trúc

![Architecture](docs/architecture.jpg)

## Chạy nhanh

### ⚡ TL;DR - Chạy được trong 5 phút

**Không database (chế độ memory):**

```bash
# 1. Clone và cài dependency
git clone https://github.com/banshee86vr/snorlx.git && cd snorlx && pnpm install

# 2. Cấu hình (sửa .env với GitHub OAuth credentials và SESSION_SECRET)
cp env.example .env

# 3. Đặt STORAGE_MODE=memory trong .env (mặc định)

# 4. Khởi động cả frontend và backend
pnpm run dev
```

**Với PostgreSQL:**

```bash
# 1. Clone và cài dependency
git clone https://github.com/banshee86vr/snorlx.git && cd snorlx && pnpm install

# 2. Chạy PostgreSQL (Docker)
docker run --name snorlx-postgres \
  -e POSTGRES_DB=snorlx \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 -d timescale/timescaledb:latest-pg16

# 3. Cấu hình (sửa .env với credentials và DATABASE_URL)
cp env.example .env

# 4. Đặt STORAGE_MODE=database và DATABASE_URL trong .env

# 5. Khởi động cả frontend và backend
pnpm run dev
```

Truy cập tại: http://localhost:5173

### Yêu cầu

- Go 1.26+
- Node.js 20+
- pnpm hoặc npm
- PostgreSQL 14+ với TimescaleDB (tùy chọn - chỉ cần cho chế độ database)
- GitHub OAuth App (xem [Hướng dẫn cấu hình](#cấu-hình-github-oauth-app))

## 🚀 Phát triển cục bộ (Không cần Docker)

### Cách 1: Chế độ Memory (Không cần database)

**Hoàn hảo để thử và phát triển!** Chạy trong vài phút, không cần dựng database.

1. **Clone và cài dependency**

```bash
git clone https://github.com/banshee86vr/snorlx.git
cd snorlx
pnpm install
```

2. **Cấu hình môi trường**

```bash
# Copy file môi trường mẫu
cp env.example .env
```

Sửa `.env` với credentials của bạn:

```env
# Storage Mode - dùng memory để chạy nhanh (không cần database)
STORAGE_MODE=memory

# GitHub OAuth (tạo tại: https://github.com/settings/developers)
GITHUB_CLIENT_ID=your_oauth_client_id
GITHUB_CLIENT_SECRET=your_oauth_client_secret

# Khóa mã hóa GitHub token đã lưu (tạo bằng: openssl rand -base64 32, tối thiểu 32 ký tự)
SESSION_SECRET=your_random_32_character_secret_string

# URLs (Vite dev server sẽ proxy /api và /ws về backend)
PORT=8080
FRONTEND_URL=http://localhost:5173
```

`DEV_MODE=true` chỉ nới lỏng kiểm tra lúc khởi động (secret placeholder, thiếu OAuth credentials); bạn vẫn cần OAuth credentials thật để đăng nhập.

3. **Khởi động ứng dụng**

```bash
# Từ thư mục gốc dự án - khởi động cả frontend và backend đồng thời
pnpm run dev
```

Ứng dụng sẽ chạy:

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080

**Hoặc khởi động riêng:**

```bash
# Chỉ backend
pnpm run dev:backend

# Chỉ frontend (terminal khác)
pnpm run dev:frontend
```

### Cách 2: Chế độ Database (Lưu trữ bền vững)

Dành cho lưu trữ dữ liệu lâu dài. Ở chế độ này backend sẽ từ chối khởi động khi không kết nối được database thay vì âm thầm rơi về memory.

#### Bước 1: Cài dependency

```bash
git clone https://github.com/banshee86vr/snorlx.git
cd snorlx
pnpm install
```

#### Bước 2: Dựng PostgreSQL + TimescaleDB

**Dùng Docker (khuyên dùng)**

```bash
docker run --name snorlx-postgres \
  -e POSTGRES_DB=snorlx \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  -d timescale/timescaledb:latest-pg16
```

**Dùng PostgreSQL cục bộ (macOS với Homebrew)**

```bash
# Cài TimescaleDB
brew install postgresql@16 timescaledb

# Khởi động PostgreSQL
brew services start postgresql@16

# Tạo database
psql postgres -c "CREATE DATABASE snorlx;"
```

#### Bước 3: Cấu hình môi trường

```bash
cp env.example .env
```

Sửa `.env`:

```env
# Storage Mode - dùng database để lưu trữ bền vững
STORAGE_MODE=database
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/snorlx?sslmode=disable

# Cấu hình GitHub
GITHUB_CLIENT_ID=your_oauth_client_id
GITHUB_CLIENT_SECRET=your_oauth_client_secret

# Khóa mã hóa GitHub token đã lưu (openssl rand -base64 32)
SESSION_SECRET=your_random_32_character_secret_string

# URLs
PORT=8080
FRONTEND_URL=http://localhost:5173
```

#### Bước 4: Khởi động ứng dụng

```bash
# Từ thư mục gốc dự án - khởi động cả frontend và backend đồng thời
# Migration tự chạy khi backend khởi động
pnpm run dev
```

Truy cập dashboard tại http://localhost:5173

## 🐳 Cài đặt bằng Docker Compose

Stack giống production với mọi dịch vụ được container hóa. Browser chỉ nói chuyện với frontend: nginx phục vụ SPA và proxy `/api` cùng `/ws` về backend — một origin duy nhất, cookie first-party và CSP nghiêm ngặt.

1. **Cấu hình môi trường**

```bash
cp env.example .env
# Đặt POSTGRES_PASSWORD, SESSION_SECRET, GITHUB_CLIENT_ID và GITHUB_CLIENT_SECRET.
# Đăng ký OAuth callback là http://localhost:5174/api/auth/callback.
# Tùy chọn: ALLOWED_GITHUB_USERS / ALLOWED_GITHUB_ORGS để hạn chế ai được đăng nhập.
```

2. **Khởi động mọi dịch vụ**

```bash
docker compose up -d
```

Dashboard có tại http://localhost:5174. Backend không public ra host; chỉ truy cập được qua proxy của frontend, và database lắng nghe trên `127.0.0.1:5433` để truy cập `psql` cục bộ.

3. **Xem log**

```bash
docker compose logs -f
```

4. **Dừng dịch vụ**

```bash
docker compose down
```

## Chấm điểm Repository

Repository được chấm điểm tổng theo phần trăm và hạng (vàng / bạc / đồng) dựa trên các kiểm tra trong 7 nhóm:

| Nhóm           | Trọng số | Ví dụ                                        |
| -------------- | -------- | -------------------------------------------- |
| Bảo mật        | 25%      | Branch protection, Dependabot, code scanning |
| Kiểm thử       | 20%      | Cấu hình test, coverage, job test trong CI   |
| CI/CD          | 15%      | Workflows, deployment, status checks         |
| Tài liệu       | 15%      | README, CONTRIBUTING, issue templates        |
| Chất lượng code| 10%      | Linters, code owners                         |
| Bảo trì        | 10%      | Hoạt động gần đây, mô tả                     |
| Cộng đồng      | 5%       | Community health files                       |

Sync một repository rồi dùng **Refresh grade** trên trang chi tiết của nó để tính hoặc cập nhật điểm. Tổng quan dashboard hiển thị điểm trung bình của các repo.

## Cấu hình GitHub OAuth App

Dự án dùng GitHub OAuth App để xác thực người dùng (đơn giản hơn GitHub Apps).

### Bước 1: Tạo OAuth App

1. Mở [GitHub Developer Settings](https://github.com/settings/developers). Với GitHub Enterprise Server, mở `https://<ghe-host>/settings/developers` trên instance đó.
2. Nhấn **OAuth Apps** → **New OAuth App**
3. Điền thông tin:

   - **Application name**: `Snorlx Dashboard` (hoặc tên bạn thích)
   - **Homepage URL**: giá trị của `FRONTEND_URL` (`http://localhost:5173` khi dev)
   - **Authorization callback URL**: `<FRONTEND_URL>/api/auth/callback` (`http://localhost:5173/api/auth/callback` khi dev; frontend proxy `/api` về backend). Nếu dùng [`BASE_PATH`](#phục-vụ-dưới-sub-path) thì là `<FRONTEND_URL><BASE_PATH>/api/auth/callback`

4. Nhấn **Register application**

### Bước 2: Lấy Credentials

1. Copy **Client ID**
2. Nhấn **Generate a new client secret** rồi copy

### Bước 3: Cấu hình môi trường

Thêm vào file `.env`:

```bash
GITHUB_CLIENT_ID=your_client_id_here
GITHUB_CLIENT_SECRET=your_client_secret_here
# Chỉ dành cho GitHub Enterprise Server. Hostname, hoặc cùng host với /api/v3.
# GITHUB_BASE_URL=https://github.mycompany.com
```

### OAuth Scopes

Ứng dụng xin các OAuth scopes sau:

- `read:user` - Đọc thông tin profile người dùng
- `user:email` - Truy cập địa chỉ email
- `repo` - Truy cập repository (cho dữ liệu workflow)
- `read:org` - Đọc tư cách thành viên tổ chức

### Giữ runs luôn mới

Runs tự cập nhật khi có người đang mở trang. Browser báo trạng thái hiển thị qua WebSocket, và backend chạy live poller cho những người có trang hiển thị: mỗi `RUN_POLL_INTERVAL` (mặc định `10s`) nó đọc lại từng run đang queued/in-progress cùng các job của run đó, và mỗi 6 chu kỳ liệt kê các run mới nhất của từng repository hiển thị để bắt các run bắt đầu từ lần sync trước. Mọi request đều conditional (`If-None-Match`) — tài nguyên không đổi trả `304`, không tốn rate-limit của GitHub và không ghi gì. Thay đổi được đẩy về browser qua WebSocket; trang vừa hiển thị lại sẽ kích hoạt một lượt quét ngay và lấy lại phần bỏ lỡ. Khi socket đứt, các trang poll backend (không bao giờ poll GitHub) tới khi kết nối lại; các request này được tính là presence trong 45 giây (áp dụng cả cho API token client như MCP). Tab nền và browser đóng nghĩa là không có traffic tới GitHub. Nút **Refresh** vẫn còn như lối thoát: chạy ngay các lượt quét conditional như bình thường. Xem [ADR 0007](docs/adr/0007-server-side-live-run-polling.md).

### Tùy chọn: Webhooks

Webhook mang thay đổi đến ngay lúc GitHub báo, và cho phép poller trả `304` cho mọi thứ nó kiểm tra. Cấu hình trong phần cài đặt repository hoặc tổ chức:

1. Vào Repository → Settings → Webhooks → Add webhook
2. **Payload URL**: `https://your-domain.com/api/webhooks/github`
3. **Content type**: `application/json`
4. **Secret**: tạo secret an toàn rồi thêm vào `.env` với tên `GITHUB_WEBHOOK_SECRET`
5. Chọn events: `Workflow runs`, `Workflow jobs`, `Deployments`

Khi webhook bật trên mọi repository, bạn có thể tăng `RUN_POLL_INTERVAL` hoặc đặt `0` để chỉ dựa vào webhook và refresh thủ công.

## Lưu trữ log (Log archive)

Snorlx có thể giữ log của job mãi mãi. Khi bật, log của mỗi job hoàn tất được tải về một lần và
lưu nén gzip vào storage, nên lịch sử trên dashboard sống sót qua giới hạn giữ log 90 ngày của
GitHub — log vẫn xem được cả khi GitHub đã xoá.

- `LOG_ARCHIVE_ENABLED` (mặc định `true`) bật/tắt archiver.
- `LOG_ARCHIVE_MAX_SIZE_BYTES` (mặc định 50 MB) giới hạn một log job chưa nén; log lớn hơn bị
  bỏ qua để một job chạy loạn không thể làm phình storage.

`GET /api/jobs/{id}/logs` trả URL nội dung cục bộ khi log đã được archive (vẫn giữ nguyên dạng
`{url}` như trước), ngược lại trả về signed URL của GitHub. Nội dung archive được phục vụ dạng
plain text qua `GET /api/jobs/{id}/logs/content`.

Các run hoàn tất trước khi bật tính năng có thể backfill trong lúc GitHub còn giữ log (tối đa
90 ngày):

```bash
curl -X POST -H "Authorization: Bearer <api-token>" http://localhost:8080/api/repositories/<id>/backfill-logs
# {"archived":123,"failed":0,"scanned":45}
```

Endpoint này đi qua các run hoàn tất của repository trong khoảng retention của GitHub, bỏ qua
job đã archive, và xử lý tối đa 500 run mỗi lần gọi — gọi lại để xử lý tiếp.

## Cấu hình

### Biến môi trường

#### Lưu trữ (Storage)

| Biến                | Bắt buộc                    | Mặc định | Mô tả                                          |
| ------------------- | --------------------------- | -------- | ---------------------------------------------- |
| `STORAGE_MODE`      | Không                       | `memory` | `memory` hoặc `database`                        |
| `DATABASE_URL`      | Chỉ khi chế độ `database`   | -        | Chuỗi kết nối PostgreSQL                        |
| `POSTGRES_PASSWORD` | Chỉ khi chế độ `database`   | -        | Mật khẩu PostgreSQL (Docker Compose dùng)       |

#### Server

| Biến                  | Mô tả                                                                                                                 | Mặc định                              |
| --------------------- | --------------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `PORT`                | Port server                                                                                                           | `8080`                                |
| `LOG_LEVEL`           | Mức log (debug, info, warn, error)                                                                                    | `info`                                |
| `LOG_FORMAT`          | `json` cho log có cấu trúc, giá trị khác cho output console                                                            | console                               |
| `SESSION_SECRET`      | Sinh khóa mã hóa GitHub token đã lưu. Bắt buộc ngoài `DEV_MODE`, tối thiểu 32 ký tự                                     | Bắt buộc                              |
| `FRONTEND_URL`        | Origin công khai (`scheme://host`) dùng cho CORS, CSRF, kiểm tra origin WebSocket và redirect OAuth. `http` thường chỉ chấp nhận cho localhost ngoài `DEV_MODE` | `http://localhost:5173` |
| `BASE_PATH`           | Đường dẫn dashboard được phục vụ, ví dụ `/snorlx`. Đặt ở cả backend và container frontend ([chi tiết](#phục-vụ-dưới-sub-path)) | trống (gốc host)               |
| `COOKIE_SECURE`       | Ép cờ `Secure` lên cookie xác thực                                                                                    | `true` khi `FRONTEND_URL` là https    |
| `TRUSTED_PROXY_CIDRS` | Các mạng (phân tách phẩy) mà `X-Forwarded-For` / `X-Real-IP` được tin (ingress, reverse proxy)                         | không (dùng địa chỉ peer)             |
| `VITE_API_URL`        | URL backend API nhúng vào build frontend. Để trống: frontend proxy `/api` và `/ws` cùng origin                         | trống                                 |

#### Phục vụ dưới sub-path

Để phục vụ dashboard tại `https://<host><base_path>` thay vì `https://<host>`, đặt `BASE_PATH=<base_path>` ở cả backend và container frontend. Không cần build lại. Với Helm chart, đặt `basePath: <base_path>`; nó cũng thêm tiền tố cho các ingress path.

- `<base_path>` bắt đầu bằng `/` và không có dấu `/` cuối. Dùng chữ, số, `-` và `_`, phân tách segment bằng `/`.
- `FRONTEND_URL` vẫn chỉ là origin (`https://<host>`).
- Đưa path vào khi đăng ký OAuth callback (`<FRONTEND_URL><base_path>/api/auth/callback`) và URL webhook (`<FRONTEND_URL><base_path>/api/webhooks/github`).
- `pnpm run dev` đọc `BASE_PATH` từ `.env` gốc; ứng dụng khi đó chỉ truy cập được tại `http://localhost:5173<base_path>`.

#### Cấu hình GitHub OAuth

| Biến                    | Mô tả                                                                                      | Bắt buộc                    |
| ----------------------- | ------------------------------------------------------------------------------------------ | --------------------------- |
| `GITHUB_CLIENT_ID`      | GitHub OAuth App Client ID                                                                 | Có                          |
| `GITHUB_CLIENT_SECRET`  | GitHub OAuth App Client Secret                                                             | Có                          |
| `GITHUB_BASE_URL`       | URL Enterprise Server (`https://host` hoặc `.../api/v3`)                                    | Không (mặc định github.com) |
| `GITHUB_WEBHOOK_SECRET` | Secret xác thực chữ ký webhook; delivery không chữ ký bị từ chối                             | Không (chỉ cho webhook)     |
| `ALLOWED_GITHUB_USERS`  | Danh sách GitHub login (phân tách phẩy) được phép đăng nhập                                  | Không (khuyên đặt)          |
| `ALLOWED_GITHUB_ORGS`   | Các GitHub organization (phân tách phẩy) có thành viên được đăng nhập                        | Không (khuyên đặt)          |
| `DEV_MODE`              | Nới lỏng kiểm tra khởi động (secret placeholder, thiếu OAuth credentials). Chỉ dùng dev cục bộ | Không                     |

#### Cấu hình Sync

| Biến                | Bắt buộc | Mặc định    | Mô tả                                                                                                          |
| ------------------- | -------- | ----------- | -------------------------------------------------------------------------------------------------------------- |
| `SYNC_LIMIT`        | Không    | `0` (tất cả)| Giới hạn số repo được sync                                                                                     |
| `SYNC_REPOS`        | Không    | -           | Danh sách repo cụ thể (phân tách phẩy) cần sync                                                                |
| `RUN_POLL_INTERVAL` | Không    | `10s`       | Tần suất refresh các run đang chạy từ GitHub cho người đang xem (conditional request). Tối thiểu `5s`, `0` tắt poller |

## API Endpoints

### Health Check

- `GET /health` - JSON `{"status":"ok","version":"<release>"}`. Liveness: chỉ kiểm tra tiến trình.
- `GET /health/ready` - `200 {"status":"ready"}` hoặc `503 {"status":"unavailable"}`. Readiness: ping luôn cả storage backend.

Tất cả route `/api` bên dưới (trừ `/api/auth/*` và webhook receiver) yêu cầu session cookie hoặc personal API token, và mọi đối tượng đều giới hạn theo các repository mà caller đã sync. Xem [SECURITY.md](SECURITY.md) để hiểu mô hình truy cập.

### Xác thực

- `GET /api/auth/login` - Bắt đầu GitHub OAuth
- `GET /api/auth/callback` - OAuth callback
- `POST /api/auth/logout` - Đăng xuất
- `GET /api/auth/status` - Kiểm tra trạng thái đăng nhập

### Organizations

- `GET /api/organizations` - Liệt kê mọi tổ chức
- `GET /api/organizations/:id` - Chi tiết một tổ chức

### Repositories

- `GET /api/repositories` - Liệt kê mọi repository
- `GET /api/repositories/:id` - Chi tiết repository
- `GET /api/repositories/scores` - Điểm mới nhất của các repository (tất cả)
- `GET /api/repositories/:id/score` - Điểm mới nhất của một repository
- `POST /api/repositories/sync` - Kích hoạt sync repository

### Workflows

- `GET /api/workflows` - Liệt kê mọi workflow
- `GET /api/workflows/:id` - Chi tiết workflow
- `PATCH /api/workflows/:id` - Cập nhật workflow
- `GET /api/workflows/:id/runs` - Các run của workflow

### Runs

- `GET /api/runs` - Liệt kê mọi run (có bộ lọc)
- `GET /api/runs/:id` - Chi tiết run
- `GET /api/runs/:id/jobs` - Các job của run
- `GET /api/runs/:id/logs` - Log của run
- `GET /api/runs/:id/annotations` - Annotations của run
- `GET /api/runs/:id/workflow-definition` - Định nghĩa YAML của workflow
- `POST /api/runs/:id/rerun` - Chạy lại workflow
- `POST /api/runs/:id/cancel` - Hủy workflow đang chạy

### Jobs

- `GET /api/jobs/:id/logs` - Log của job
- `GET /api/jobs/:id/logs/content` - Nội dung log đã archive (plain text)

### Dashboard

- `GET /api/dashboard/summary` - Tổng quan dashboard
- `GET /api/dashboard/trends` - Dữ liệu xu hướng

### Real-time

- `GET /ws` - Endpoint WebSocket cho cập nhật realtime. Server events: `workflow_run` (một run đã lưu đổi hoặc xuất hiện), `workflow_job` (`{run_id, run_github_id}`: các job của run đó thay đổi), `deployment`, `sync:*`. Client gửi `presence` (`{"active": bool}`) kèm trạng thái hiển thị trang; live poller chỉ chạy cho người dùng có trang hiển thị

### Webhooks

- `POST /api/webhooks/github` - Webhook receiver của GitHub

## Cấu trúc dự án

```
├── .github/workflows/      # CI và Security GitHub Actions
├── frontend/               # Frontend React
│   ├── nginx/              # nginx template (SPA + proxy cùng origin cho /api và /ws)
│   ├── src/
│   │   ├── components/     # UI components (layout, protected routes)
│   │   ├── context/        # React contexts (auth, theme, socket, sync, sidebar)
│   │   ├── lib/            # Hàm tiện ích
│   │   ├── pages/          # Các trang
│   │   ├── services/       # API client
│   │   ├── styles/         # Styles toàn cục
│   │   ├── test/           # Setup test
│   │   └── types/          # Kiểu TypeScript
│   └── ...
├── backend/                # Backend Go
│   ├── cmd/server/         # Điểm khởi động chính
│   ├── internal/
│   │   ├── config/         # Cấu hình và validation
│   │   ├── github/         # GitHub client
│   │   ├── handlers/       # HTTP handlers (authn, authz theo user)
│   │   ├── httpmiddleware/ # Trusted-proxy client IP, request logging
│   │   ├── models/         # Data models
│   │   ├── scorer/         # Chấm điểm repository (vàng/bạc/đồng)
│   │   ├── storage/        # Lớp lưu trữ (memory/database) và migrations
│   │   ├── tokencrypt/     # Mã hóa GitHub token đã lưu
│   │   └── websocket/      # WebSocket hub cho cập nhật realtime theo user
│   └── ...
├── docs/adr/               # Ghi chú quyết định kiến trúc
├── helm/                   # Kubernetes Helm charts
└── docker-compose.yml      # Cấu hình Docker
```

## Xử lý sự cố

### Lỗi thường gặp

**Backend không khởi động: thiếu GitHub OAuth credentials**

- Cung cấp `GITHUB_CLIENT_ID` và `GITHUB_CLIENT_SECRET` từ [GitHub OAuth App](#cấu-hình-github-oauth-app)
- `DEV_MODE=true` cho phép tiến trình khởi động thiếu chúng để làm việc cục bộ với code không liên quan auth, nhưng không ai đăng nhập được

**Backend không khởi động: SESSION_SECRET hoặc FRONTEND_URL bị từ chối**

- `SESSION_SECRET` phải là giá trị ngẫu nhiên ít nhất 32 ký tự (`openssl rand -base64 32`)
- `FRONTEND_URL` phải là origin dạng `https://dash.example.com`; `http` thường chỉ chấp nhận cho localhost trừ khi `DEV_MODE=true`

**Lỗi kết nối database**

- Kiểm tra PostgreSQL đang chạy: `psql -U postgres -d snorlx`
- Kiểm tra `DATABASE_URL` trong `.env` khớp credentials database
- Đảm bảo extension TimescaleDB đã cài
- Ở chế độ database, backend thoát nếu mất kết nối — cố ý vậy; `/health/ready` trả 503 khi database mất

**Đăng nhập xong nhưng dashboard trống**

- Mỗi người dùng chỉ thấy các repository họ tự sync bằng token của mình. Nhấn **Sync** một lần sau khi đăng nhập (và một lần sau khi nâng cấp từ release trước 1.1)

**403 trên mọi thao tác hoặc WebSocket đứt kết nối**

- Origin của browser phải bằng `FRONTEND_URL`. Phục vụ SPA và API từ cùng origin đó (image frontend proxy `/api` và `/ws`); đừng trỏ browser thẳng vào port backend

**Port đã bị chiếm (EADDRINUSE)**

- Port Backend (8080) hoặc Frontend (5173) đã có tiến trình khác chiếm
- Tìm tiến trình: `lsof -i :8080` hoặc `lsof -i :5173`
- Tắt tiến trình: `kill -9 <PID>`
- Hoặc đổi port trong `.env`

**Chế độ memory mất dữ liệu**

- Đúng như dự kiến! Chế độ memory không giữ dữ liệu qua các lần khởi động lại
- Dùng `STORAGE_MODE=database` để lưu trữ bền vững

**Lỗi "Too many requests"**

- Rate limiting đang bật. Chờ rồi thử lại
- Ở production, cơ chế này chống lạm dụng

## CI/CD

Dự án gồm ba GitHub Actions workflows:

### CI (`ci.yml`)

Chạy trên push và pull request tới `main`:

- **Backend**: Go vet, test với race detector và coverage, build binary
- **Frontend**: Lint, test với coverage, build production
- **Helm**: `helm lint` và `helm template`, bao gồm fallback image tag từ `appVersion` của chart

### Release (`release.yml`)

Chạy trên push tới `main`. release-please mở hoặc cập nhật release pull request và publish release khi PR đó merge. Khi publish nó xác minh release bất biến, build image backend và frontend, và đẩy Helm chart. Xem [Releases](#releases).

### Security (`security.yml`)

Chạy trên push, pull request tới `main`, và hàng tuần (thứ Hai 08:00 UTC):

- **CodeQL Analysis**: phân tích tĩnh cho Go và JavaScript/TypeScript
- **Trivy Scans**: quét lỗ hổng filesystem cùng quét Docker image cho backend và frontend; các phát hiện CRITICAL và HIGH fixable sẽ làm job fail
- **Dependency Review**: gắn cờ dependency mới có lỗ hổng trên PR
- **Go Security**: govulncheck và gosec (pin version) cho lỗ hổng riêng của Go
- **npm Audit**: kiểm tra lỗ hổng trong các package Node

Mọi workflow pin actions tới full commit SHA và cài dependency với `--frozen-lockfile`; Dependabot giữ các pin, image digest và lockfile luôn mới. Xem [ADR 0006](docs/adr/0006-supply-chain-pinning-and-signing.md).

## Tuân thủ 12-Factor App

Ứng dụng tuân theo [phương pháp 12-Factor](https://12factor.net/):

1. **Codebase**: một repo duy nhất, theo dõi bằng Git
2. **Dependencies**: khai báo tường minh trong `go.mod` và `package.json`
3. **Config**: mọi cấu hình qua biến môi trường
4. **Backing services**: database là tài nguyên gắn kèm qua URL
5. **Build, release, run**: Docker image tag theo phiên bản sản phẩm
6. **Processes**: stateless; session nằm trong storage
7. **Port binding**: HTTP server tự chứa
8. **Concurrency**: scale ngang qua replicas
9. **Disposability**: tắt sạch sẽ khi nhận SIGTERM
10. **Dev/prod parity**: Docker Compose đối chiếu production
11. **Logs**: JSON ra stdout, thu thập bởi nền tảng
12. **Admin processes**: migration chạy cùng lúc khởi động

## Releases

Release được cắt bằng [release-please](https://github.com/googleapis/release-please) từ Conventional Commits trên `main`. Nó mở release pull request bump phiên bản sản phẩm trong các file `package.json` của gốc, frontend và MCP, `backend/internal/version/version.go`, `mcp/src/server.ts` và `helm/snorlx/Chart.yaml`, đồng thời ghi `CHANGELOG.md`. Merge PR đó sẽ publish tag `vX.Y.Z` và một GitHub Release.

Repository dùng GitHub immutable releases. Sau khi publish, tag không thể di chuyển, xoá hay dùng lại, và GitHub gắn release attestation. Kiểm tra bằng `gh release verify vX.Y.Z`. Job verify, image hoặc chart fail có thể chạy lại từ Actions. Một lần chạy mới sẽ publish tag `vX.Y.Z` đã có trên commit đó, hoặc tag bạn truyền vào workflow_dispatch. Release sai cần một patch version mới.

Image chỉ publish theo phiên bản, cho `linux/amd64` và `linux/arm64`:

```bash
docker pull ghcr.io/banshee86vr/snorlx-backend:X.Y.Z
docker pull ghcr.io/banshee86vr/snorlx-frontend:X.Y.Z
```

Helm chart là OCI artifact. Tag image trống trong chart dùng `appVersion`, khớp với image tag. Chart cần origin công khai của dashboard: bật ingress với TLS (origin thành `https://<first host>`) hoặc đặt `backend.frontendUrl` tường minh, ví dụ khi TLS kết thúc trước ingress hoặc để truy cập port-forward. Các secret được sinh (mật khẩu database, `SESSION_SECRET`) tạo lúc cài đầu tiên và giữ qua các lần nâng cấp.

```bash
helm install snorlx oci://ghcr.io/banshee86vr/charts/snorlx --version X.Y.Z \
  --set ingress.enabled=true \
  --set 'ingress.hosts[0].host=snorlx.example.com' \
  --set 'ingress.tls[0].secretName=snorlx-tls' \
  --set 'ingress.tls[0].hosts[0]=snorlx.example.com' \
  --set github.clientId=... --set github.clientSecret=... \
  --set github.allowedOrgs=my-org
```

Image và chart được ký bằng keyless cosign và mang SLSA provenance cùng SBOM:

```bash
cosign verify \
  --certificate-identity-regexp 'https://github.com/banshee86vr/snorlx/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/banshee86vr/snorlx-backend:X.Y.Z
```

release-please xác thực bằng GitHub App. Tạo app (Contents, Pull requests và Issues write), cài lên repository này, và lưu `RELEASE_PLEASE_APP_ID` cùng `RELEASE_PLEASE_APP_PRIVATE_KEY` vào secrets. Bật immutable releases trong Settings, General, Releases trước release đầu tiên.

Mọi Conventional Commit, kể cả `chore(deps)`, đều mở hoặc cập nhật patch release pull request. Merge khi bạn muốn ship.

## MCP

Snorlx cung cấp Model Context Protocol server cho các agent (Cursor, v.v.). Tạo personal API token trong **Settings → API tokens**, rồi xem [docs/mcp.md](docs/mcp.md) để cấu hình stdio và Streamable HTTP.

## License

MIT License - xem [LICENSE](LICENSE).
