# Snorlx MCP

Kết nối Cursor (hoặc bất kỳ MCP client nào) vào một instance Snorlx đang chạy bằng personal API token.

## Yêu cầu trước

1. Backend Snorlx đang chạy (mặc định `http://localhost:8080`)
2. Đăng nhập dashboard và mở **Settings → API tokens**
3. Tạo token. Ưu tiên **read-only** trừ khi bạn cần các write tool (sync, rerun, cancel). Copy một lần duy nhất; sau đó không hiển thị lại.

Tiến trình MCP hành động với danh tính người tạo token: nó chỉ thấy đúng các repository mà người đó đã sync, không hơn.

## Cài đặt

Từ thư mục gốc repo:

```bash
pnpm install
pnpm mcp:build
```

## Cursor cục bộ (stdio)

Thêm vào cấu hình MCP của Cursor (ví dụ `~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "snorlx": {
      "command": "node",
      "args": ["/absolute/path/to/snorlx/mcp/dist/index.js"],
      "env": {
        "SNORLX_API_URL": "http://localhost:8080",
        "SNORLX_API_TOKEN": "snorlx_..."
      }
    }
  }
}
```

## Product / remote (Streamable HTTP)

Chế độ HTTP yêu cầu credential client **riêng** (`MCP_HTTP_TOKEN`) và mặc định bind vào localhost.

```bash
SNORLX_API_URL=https://snorlx.example.com \
SNORLX_API_TOKEN=snorlx_... \
MCP_HTTP_TOKEN="$(openssl rand -hex 32)" \
MCP_HOST=127.0.0.1 \
MCP_PORT=3100 \
pnpm mcp:http
```

| Biến | Mục đích |
|----------|---------|
| `SNORLX_API_TOKEN` | Personal API token mà tiến trình MCP dùng để gọi Snorlx |
| `MCP_HTTP_TOKEN` | Bắt buộc. Client phải gửi `Authorization: Bearer <MCP_HTTP_TOKEN>` vào `/mcp` |
| `MCP_HOST` | Địa chỉ bind (mặc định `127.0.0.1`). Chỉ đặt `0.0.0.0` khi đứng sau reverse proxy có xác thực / mTLS |
| `MCP_PORT` | Port lắng nghe (mặc định `3100`) |
| `MCP_ALLOWED_HOSTS` | Các giá trị header `Host` (phân tách phẩy) được chấp nhận trên `/mcp` (chống DNS rebinding). Mặc định là các tên loopback khi `MCP_HOST` là loopback; đặt khi bind chỗ khác |
| `MCP_SESSION_TTL_SECONDS` | Session MCP nhàn rỗi bị đóng sau số giây này (mặc định 1800) |

Không có `MCP_HTTP_TOKEN`, bất kỳ ai truy cập được `/mcp` đều có thể điều khiển mọi tool bằng token Snorlx của tiến trình. Hãy coi `/mcp` như mặt phẳng điều khiển riêng tư: giữ trên loopback, hoặc kết thúc TLS và xác thực ở reverse proxy trước khi mở ra mạng. Lỗi request trả về `500` chung chung; chi tiết chỉ ghi vào log của tiến trình MCP.

## Tools

Các read tool bao phủ dashboard summary/trends, tổ chức, repo, điểm số, workflows, runs, jobs, annotations, active pipelines, và **nội dung log**:

- `get_job_logs` trả **nội dung** log của một job — lấy từ archive của Snorlx kể cả khi GitHub đã xoá sau 90 ngày retention, với phương án dự phòng là log live trên GitHub.
- `get_run_logs` trả log của một run dạng một khối text: log các job đã hoàn thành nối với nhau bằng header `### <name> (job <id>)`. Ưu tiên `get_run_annotations` khi cần gợi ý lỗi nhanh; cả hai đều cắt theo `max_chars` (mặc định 24k) cho giới hạn ngữ cảnh của MCP.

Các write tool (yêu cầu token scope `write`; xác nhận với người dùng trước khi dùng):

- `sync_repositories` / `sync_repository`
- `backfill_deployment_runs`
- `backfill_repository_logs` — archive log job của các run hoàn thành gần đây vào Snorlx để sống sót qua khoảng retention của GitHub (tốn nhiều rate limit; gọi lại để xử lý tiếp)
- `update_workflow`
- `rerun_workflow` / `cancel_run`

## Lưu ý bảo mật

- Token được hash khi lưu; plaintext chỉ hiển thị đúng một lần lúc tạo.
- Bearer API token hợp lệ bỏ qua CSRF; session cookie vẫn kiểm tra Origin. Header `Authorization` giả không vượt qua được CSRF.
- Không bao giờ đặt OAuth client secret hay GitHub token vào biến môi trường của MCP; chỉ dùng API token `snorlx_` do người dùng tạo.
- Chế độ stdio giữ secret trong env của tiến trình Cursor cục bộ; đừng commit chúng.
- Mọi HTTP client dùng chung danh tính chủ token. Tạo một token cho mỗi agent hoặc mỗi người để thu hồi tập trung đúng chỗ.
