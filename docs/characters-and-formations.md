# Character Và Formation

## Level, Experience Và Giới Hạn Character

Mỗi player trong match có progression runtime, khởi đầu ở level `1` với experience `0`. Progression chưa được lưu vào storage và chưa có nguồn nhận experience.

Giới hạn character hiện tại được tính bằng `min(level, 9)`: level 1 có 1 slot, level 2 có 2 slot và từ level 9 trở lên có 9 slot. Hằng `strategy.MaxCharacters = 9` là giới hạn tuyệt đối của formation. Bảng experience nằm trong `modules/game/core/progression/level_progression.json`; mỗi giá trị là experience cần thêm để sang level kế tiếp.

## Character Lifecycle

Player mới được tạo cùng một character có weapon chọn ngẫu nhiên từ embedded catalog. Character ID có dạng `<user_id>:<sequence>` và ổn định khi array được compact.

Character có position, target position, health/max health, damage, movement/attack stats, weapon và combat state. Khi `Health <= 0`, character bị remove trong combat tick; formation được gán lại trước snapshot tiếp theo.

Mỗi player tối đa `9` character non-nil. `nil` không chiếm slot và không xuất hiện trong snapshot.

## Weapon Catalog

Catalog mặc định nằm tại [`weapons.json`](../modules/game/core/entity/weapons.json). Weapon thay thế toàn bộ gameplay stats của character khi equip.

| Field | Ý nghĩa |
| --- | --- |
| `type` | Loại weapon gameplay |
| `name` | Stable key để Unity đối chiếu asset/reference |
| `range_class` | `melee` hoặc `ranged` |
| `health` | Max health sau khi equip |
| `damage` | Sát thương gốc cố định của mỗi đòn |
| `crit_chance`, `crit_multiplier` | Xác suất chí mạng và hệ số nhân sát thương gốc |
| `damage_reduction` | Tỉ lệ giảm sát thương khi nhận đòn, tối đa 60% |
| `move_speed` | World units mỗi giây |
| `attack_speed` | Số attack cycle mỗi giây |
| `attack_range` | Bán kính khóa target |
| `impact_ratio` | Vị trí impact trong attack cycle |
| `projectile_speed` | World units mỗi giây; ranged phải lớn hơn 0 |
| `regen_rate` | Stat đã có, chưa có regen system |

`spear`, `sword`, `axe`, `blunt` là melee. `bow`, `staff`, `wand` là ranged.

## Strategy 5x5

Strategy là mask cố định `5x5`: `1` mở slot, `0` khóa slot. Tâm là cell `[2][2]`, khoảng cách slot là `1.5 world units`. Catalog có `x-type` (hai đường chéo) và `plus-type` (hàng và cột giữa), mỗi strategy mở 9 slot đối xứng quanh tâm. `x-type` có offset xa nhất khoảng `4.24 units`; `plus-type` có offset xa nhất `3 units`.

Slot được sắp từ gần tâm ra xa, hòa thì theo row/column. Character được stable-sort theo class:

1. `ranged` gần tâm.
2. `melee` phía ngoài.

Các character cùng class giữ thứ tự trong `Player.Characters`.

## Formation Movement

`Player.Position` là tâm formation. Offset slot xoay theo `Player.Facing`; facing `(1,0)` là hướng mặc định và right vector là `(facing.Y, -facing.X)`.

Mỗi tick server tính lại `TargetPosition`, sau đó character dùng MoveTowards với tốc độ:

```text
character.MoveSpeed * 1.5
```

Character không overshoot và không teleport khi formation/facing đổi. Character đầu tiên khi player spawn được snap vào slot khởi tạo.

## Character Box

Match duy trì Character Box tĩnh và mỗi phút refill đến target của mode. Box chứa một weapon type được chọn đều từ weapon catalog. Khi player đi vào bán kính collision, server khóa một box cho một player và bắt đầu countdown; mỗi player chỉ được claim một box. Player phải ở trong vùng, còn sống và còn capacity theo level cho tới deadline. Delay dựa trên số character lúc bắt đầu claim: count 1–4 chờ tương ứng 1–4 giây, 5–7 chờ 5 giây và 8 chờ 6 giây. Player đã đạt giới hạn runtime không thể claim; giới hạn tuyệt đối là 9 character. Bảng thời gian nằm trong `modules/game/core/characterbox/pickup_delays.json`.

Opcode reliable `106` broadcast `PICKUP_STARTED` và `PICKUP_CANCELLED`; client tự tính countdown từ deadline tick. Khi hoàn tất, server mới random skin, grant character và gửi `DESPAWNED`. Box đang được claim vẫn tồn tại trong world và được tính khi refill.

## Giới Hạn Hiện Tại

- Client chưa có opcode chọn strategy; mọi player dùng mặc định `x-type`.
- Character chưa persist qua match restart.
- Chưa có collision giữa character, obstacle hoặc blocked strategy cell.
- `impact_ratio` hiện là dữ liệu tạm, chưa hiệu chỉnh theo animation Unity cuối cùng.
