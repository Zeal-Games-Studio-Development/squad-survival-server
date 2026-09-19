# Squad Survival Server

Backend authoritative cho game Squad Survival, xây dựng bằng Nakama Go runtime module, PostgreSQL và Protobuf.

## Quick Start

```powershell
Copy-Item .env.example .env
docker compose up --build -d
docker compose logs -f nakama
```

Các endpoint local mặc định:

- Nakama API/WebSocket: `http://localhost:7350`
- Nakama Console: `http://localhost:7351`
- PostgreSQL: `localhost:5432`

Thay toàn bộ giá trị `change-me` trong `.env` trước khi triển khai ra ngoài máy local.

## Development

```powershell
go test -count=1 ./...
go vet ./...
buf generate
```

Đọc [Developer Handbook](docs/README.md) để xem kiến trúc, gameplay, protocol và quy trình vận hành. Các hạng mục chưa triển khai được theo dõi trong [TODO.md](TODO.md).
