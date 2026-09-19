# Kiểm Thử Và Code Generation

## Go Tests

Chạy toàn bộ test không dùng cache:

```powershell
go test -count=1 ./...
```

Test hiện bao phủ:

- Movement normalization, sequence, timeout và world clamp.
- Character stats, weapon config, damage range và stable ID.
- Strategy parsing, capacity, slot order, rotation và soft movement.
- Spatial insert/move/remove/query, tọa độ âm và detection boundary.
- Combat target selection, movement gate, attack timing, simultaneous damage và death.
- Projectile spawn, movement, hit, expiry và protobuf lifecycle events.
- Match join/leave, capacity, spatial synchronization và personalized broadcasts.
- Matchmaking defaults, query và backfill behavior.

Static analysis:

```powershell
go vet ./...
```

## Protobuf Generation

```powershell
buf generate
```

Lệnh generate:

- Go code cạnh source `.proto` để Docker compile vào plugin.
- C# code local tại `clients/unity/Generated/Protobuf` để copy sang Unity repo.

Sau khi generate, kiểm tra thay đổi:

```powershell
git diff --check
git status --short
```

Không sửa thủ công generated `.pb.go` hoặc `.cs`. Khi schema thay đổi, cập nhật `.proto`, generate lại cả hai target và chạy test.

## Docker Verification

```powershell
docker compose build nakama
docker compose up -d
docker compose logs nakama
```

Log cần xác nhận Go runtime module load thành công và match/RPC handlers được đăng ký. Docker build không chạy Buf; nếu quên generate `.pb.go`, image sẽ compile schema cũ hoặc build thất bại tùy thay đổi source.

## Documentation Audit

Khi constants hoặc protocol thay đổi, tìm các giá trị liên quan trong `docs/` và đối chiếu source:

```powershell
rg -n "opcode|tick|radius|capacity|projectile|impact_ratio" docs
```

Không mô tả generated client files bằng đường dẫn Git vì `clients/` bị ignore. Luôn liên kết tới `.proto` làm contract chính.
