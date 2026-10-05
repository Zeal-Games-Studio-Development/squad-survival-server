# AI player

Survival và battle royale tạo bot ngay trong `MatchInit`. Bot dùng cùng `entity.Player`, random spawn, character/weapon mặc định, skin fallback, spatial grid, progression và combat của người chơi. Logic quyết định di chuyển nằm riêng trong `modules/game/core/ai.Controller`; `Player` không chứa AI behaviour.

## Cấu hình

Chỉnh `modules/game/core/ai/config.json`:

```json
{
  "survival_count": 8,
  "battle_royale_count": 16,
  "prefixes": ["Shadow", "Iron", "Swift", "Wild", "Silent", "Crimson", "Frost", "Storm"],
  "suffixes": ["Wolf", "Raven", "Hunter", "Blade", "Fox", "Ghost", "Falcon", "Knight"]
}
```

Count phải không âm; `0` tắt bot của mode đó. Nếu có bot, hai danh sách tên phải có phần tử và không chứa chuỗi trắng. Tên có dạng `Iron Wolf`; khi trùng trong match, thêm số như `Iron Wolf 2`. Config được nhúng bằng `go:embed`: cần build và deploy/restart server để dùng cấu hình mới. Không có hot reload hoặc spawn bù bot chết.

## Identity và network

- `UserID`: `ai-user:<match-id>:<index>`, index bắt đầu từ 1.
- `SessionID`: `ai-session:<match-id>:<index>`.
- Character ID vẫn theo quy tắc `<user-id>:<character-sequence>`.
- ID AI là định danh gameplay nội bộ, không phải account hay session Nakama. Bot không có presence, inventory storage hoặc active match membership.
- Client nhận diện bằng prefix ID; không có trường protobuf mới. Roster, movement, progression và combat vẫn gửi bot tới người thật theo vùng quan sát hiện có.

`Players` chứa cả bot và người thật; `AIControllers` ánh xạ session ID tới controller. `player_count` trong label và state snapshot chỉ đếm người thật. Mỗi match nhận tối đa 32 người thật, cộng thêm số bot cấu hình. Reservation, match đầy và TTL không bị bot chiếm chỗ hoặc giữ sống.

## Hành vi và vòng đời

Mỗi tick gameplay, controller tìm mục tiêu trong bán kính 120 đơn vị theo thứ tự: box gần nhất nếu roster chưa đầy theo level, XP gần nhất nếu chưa đạt level tối đa, rồi đối thủ còn sống gần nhất (người thật hoặc bot). Không tìm thấy mục tiêu thì đi ngẫu nhiên, đổi hướng mỗi 30 tick (3 giây), quay vào trong khi gần biên map. Mục tiêu được đánh giá lại mỗi tick; vật phẩm biến mất hoặc đối thủ bị loại sẽ được bỏ qua.

Bot đứng lại khi vào bán kính nhặt box để hoàn tất thời gian nhặt. Hướng cuối cùng được giảm độ dài để tránh đi quá mục tiêu. Controller chỉ đặt `Direction`/`Facing`; tốc độ, formation, nhặt đồ, damage và XP được tính bằng hệ thống gameplay chung. Bot bị loại dừng di chuyển và giữ entity theo cơ chế player hiện tại.

Survival chạy AI từ tick đầu tiên. Battle royale giữ bot bất động trong `waiting`; người thật đầu tiên kích hoạt countdown, đủ 32 người thật hoặc hết countdown mới vào `playing`. AI dừng khi match kết thúc. Match không có người thật vẫn đóng theo vòng đời hiện có dù còn bot.

## Kiểm thử

Chạy `go test ./...`. Test AI kiểm tra config/tên/ID, ưu tiên và thay đổi mục tiêu, nhặt đồ/XP/combat, vùng spawn, replication cho người thật, 32 slot người thật, và lifecycle của cả hai mode.
