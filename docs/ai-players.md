# AI player

Survival và battle royale tạo bot ngay trong `MatchInit`. Bot dùng cùng `entity.Player`, random spawn, character/weapon mặc định, spatial grid, progression và combat của người chơi. Logic quyết định di chuyển nằm riêng trong `modules/game/core/ai.Controller`; `Player` không chứa AI behaviour.

## Chọn vũ khí cho bot

Nhân vật đầu tiên của mỗi bot chọn ngẫu nhiên đều một `weapon_id` từ toàn bộ catalog trong [`weapons.json`](../modules/game/core/entity/weapons.json), gồm cả các biến thể. Lựa chọn này độc lập với `squad_loadout` của bot.

Bot không có inventory tài khoản. `squad_loadout` mặc định lấy `weapon_id` đầu tiên của mỗi `type` theo thứ tự trong catalog; nó không được random. Khi bot nhặt Character Box, `weapon_type` của box xác định slot, rồi server lấy `weapon_id` trong slot đó để tạo nhân vật mới. Vì vậy nhân vật đầu tiên có thể dùng biến thể, còn nhân vật nhận từ box dùng ID đã chọn sẵn trong loadout mặc định.

## Cấu hình

Chỉnh `modules/game/core/ai/config.json`:

```json
{
  "survival_count": 8,
  "battle_royale_count": 16,
  "disengage_difference_by_max_characters": {
    "1": 1, "2": 1, "3": 1, "4": 1, "5": 1,
    "6": 2, "7": 2, "8": 2, "9": 3
  },
  "prefixes": ["Shadow", "Iron", "Swift", "Wild", "Silent", "Crimson", "Frost", "Storm"],
  "suffixes": ["Wolf", "Raven", "Hunter", "Blade", "Fox", "Ghost", "Falcon", "Knight"]
}
```

Count phải không âm; `0` tắt bot của mode đó. Nếu có bot, hai danh sách tên phải có phần tử và không chứa chuỗi trắng. Tên có dạng `Iron Wolf`; khi trùng trong match, thêm số như `Iron Wolf 2`. Config được nhúng bằng `go:embed`: cần build và deploy/restart server để dùng cấu hình mới. Không có hot reload hoặc spawn bù bot chết.

`disengage_difference_by_max_characters` đặt ngưỡng bỏ đuổi `n` theo sức chứa tối đa của AI ở level hiện tại. Key hợp lệ từ 1 đến 9, giá trị từ 1 đến key. Mức không cấu hình dùng `max(1, floor(MaxCharacters / 3))`: sức chứa 1–5 dùng 1, 6–8 dùng 2 và 9 dùng 3. `NewController(player)` dùng config mặc định; `NewControllerWithConfig(player, config)` dùng config truyền vào. Hai mode truyền config đã đọc khi spawn cho controller.

## Identity và network

- `UserID`: `ai-user:<match-id>:<index>`, index bắt đầu từ 1.
- `SessionID`: `ai-session:<match-id>:<index>`.
- Character ID vẫn theo quy tắc `<user-id>:<character-sequence>`.
- ID AI là định danh gameplay nội bộ, không phải account hay session Nakama. Bot không có presence hoặc active match membership.
- Client nhận diện bằng prefix ID; không có trường protobuf mới. Roster, movement, progression và combat vẫn gửi bot tới người thật theo vùng quan sát hiện có.

`Players` chứa cả bot và người thật; `AIControllers` ánh xạ session ID tới controller. `player_count` trong label và state snapshot chỉ đếm người thật. Mỗi match nhận tối đa 32 người thật, cộng thêm số bot cấu hình. Reservation, match đầy và TTL không bị bot chiếm chỗ hoặc giữ sống.

## Hành vi và vòng đời

Mỗi tick gameplay, controller quét spatial grid trong bán kính 120 đơn vị. Khi chưa đuổi ai, bot ưu tiên đối thủ gần nhất có số character còn sống không vượt quá số quân của bot, gồm người thật và bot khác, trước box và XP. Chỉ character có `Health > 0` được tính khi so sánh.

Từ lúc bắt đầu đuổi player, bot giữ mục tiêu đó và tiếp tục đuổi khi `quân địch − quân AI < n`. Bot tra lại `n` theo sức chứa tối đa hiện tại mỗi tick. Khi chênh lệch đạt hoặc vượt `n`, đối thủ chết, bị xóa hoặc ra ngoài bán kính 120, bot bỏ mục tiêu và tìm đối thủ phù hợp khác. Ví dụ sức chứa 3, `n=1`: bot có 3 quân đuổi đối thủ 3 quân; nếu bot mất một quân thì bỏ đuổi.

Khi không có đối thủ phù hợp, bot chọn box gần nhất nếu đội hình chưa đầy theo level, rồi XP gần nhất nếu chưa đạt level tối đa. Không có mục tiêu phù hợp thì đi ngẫu nhiên, đổi hướng mỗi 30 tick (3 giây), quay vào trong khi gần biên map. Bỏ đuổi chỉ đổi hướng di chuyển; không thêm chạy trốn, cooldown hoặc tắt combat tự động.

Bot đứng lại khi vào bán kính nhặt box để hoàn tất thời gian nhặt. Hướng cuối cùng được giảm độ dài để tránh đi quá mục tiêu. Controller chỉ đặt `Direction`/`Facing`; tốc độ, formation, nhặt đồ, damage và XP được tính bằng hệ thống gameplay chung. Bot bị loại dừng di chuyển và giữ entity theo cơ chế player hiện tại.

Cả Survival và Battle Royale giữ bot bất động trong `waiting` và chỉ chạy AI khi `playing`. Người thật đầu tiên kích hoạt countdown; Survival chuyển sang `playing` khi đủ 3 người hoặc hết 30 giây, Battle Royale khi đủ 32 người hoặc hết 15 giây. AI dừng khi match vào `ended`. Match không có người thật vẫn đóng theo vòng đời của mode dù còn bot.

## Kiểm thử

Chạy `go test ./...`. Test AI kiểm tra config/tên/ID, chọn đối thủ theo số quân còn sống, giữ/bỏ mục tiêu theo ngưỡng và level, chuyển sang box/XP/wander, nhặt đồ/XP/combat, vùng spawn, replication cho người thật, 32 slot người thật, và lifecycle của cả hai mode.
