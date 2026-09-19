# Cài Đặt Và Chạy Local

## Yêu Cầu

- Docker Engine hoặc Docker Desktop có Docker Compose.
- Go `1.26.5` nếu chạy test ngoài container.
- Buf nếu cần generate lại Protobuf; Nakama runtime không cần Buf hoặc `protoc`.

## Environment

Tạo `.env` từ file mẫu:

```powershell
Copy-Item .env.example .env
```

| Biến | Mục đích |
| --- | --- |
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` | Database Nakama |
| `NAKAMA_SERVER_KEY` | Xác thực Nakama client/server |
| `NAKAMA_SESSION_ENCRYPTION_KEY` | Ký access token |
| `NAKAMA_SESSION_REFRESH_ENCRYPTION_KEY` | Ký refresh token |
| `NAKAMA_RUNTIME_HTTP_KEY` | Xác thực server-to-server runtime HTTP |
| `NAKAMA_CONSOLE_USERNAME`, `NAKAMA_CONSOLE_PASSWORD` | Đăng nhập Console |
| `NAKAMA_CONSOLE_SIGNING_KEY` | Ký session Console |

Không commit `.env`. Các encryption/signing key phải cố định giữa những lần restart nếu muốn token hiện có tiếp tục hợp lệ.

## Docker Compose

```powershell
docker compose up --build -d
docker compose ps
docker compose logs -f nakama
```

Nakama chờ PostgreSQL healthy, chạy migration rồi load `backend.so`. PostgreSQL sử dụng named volume `nakama_postgres_data`.

| Port | Dịch vụ |
| --- | --- |
| `5432` | PostgreSQL |
| `7349` | Nakama gRPC |
| `7350` | Nakama HTTP và WebSocket |
| `7351` | Nakama Console |

Dừng service mà vẫn giữ database:

```powershell
docker compose down
```

Không dùng `docker compose down -v` nếu cần giữ PostgreSQL data.

## Build Và Health

```powershell
docker compose build nakama
docker compose up -d
docker compose ps
```

Compose healthcheck gọi `nakama healthcheck`. Module còn đăng ký RPC `healthcheck`, trả JSON `{"status":"ok"}` để kiểm tra runtime module đã load.

## Giới Hạn Hiện Tại

- Match state nằm trong memory; restart Nakama làm mất trận đang chạy.
- Redis và reconnect grace period chưa được triển khai.
- Docker image chỉ compile generated `.pb.go`; code generation phải chạy trước Docker build khi schema thay đổi.
