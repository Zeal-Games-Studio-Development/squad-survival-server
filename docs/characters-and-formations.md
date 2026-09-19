# Character Và Formation

## Character Lifecycle

Player mới được tạo cùng một character có weapon chọn ngẫu nhiên từ embedded catalog. Character ID có dạng `<user_id>:<sequence>` và ổn định khi array được compact.

Character có position, target position, health/max health, damage, movement/attack stats, weapon và combat state. Khi `Health <= 0`, character bị remove trong combat tick; formation được gán lại trước snapshot tiếp theo.

Mỗi player tối đa `25` character non-nil. `nil` không chiếm slot và không xuất hiện trong snapshot.

## Weapon Catalog

Catalog mặc định nằm tại [`weapons.json`](../modules/game/core/entity/weapons.json). Weapon thay thế toàn bộ gameplay stats của character khi equip.

| Field | Ý nghĩa |
| --- | --- |
| `type` | Loại weapon gameplay |
| `name` | Stable key để Unity đối chiếu asset/reference |
| `range_class` | `melee` hoặc `ranged` |
| `health` | Max health sau khi equip |
| `damage`, `damage_ratio` | Base damage và độ lệch random |
| `move_speed` | World units mỗi giây |
| `attack_speed` | Số attack cycle mỗi giây |
| `attack_range` | Bán kính khóa target |
| `impact_ratio` | Vị trí impact trong attack cycle |
| `projectile_speed` | World units mỗi giây; ranged phải lớn hơn 0 |
| `regen_rate` | Stat đã có, chưa có regen system |

`dagger`, `spear`, `sword`, `axe`, `blunt` là melee. `bow`, `staff`, `wand` là ranged.

## Strategy 5x5

Strategy là mask cố định `5x5`: `1` mở slot, `0` khóa slot. Tâm là cell `[2][2]`, khoảng cách slot là `1.5 world units`. Catalog hiện chỉ có strategy `compact` với 25 slot mở.

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

## Giới Hạn Hiện Tại

- Client chưa có opcode chọn strategy; mọi player dùng `compact`.
- Character chưa persist qua match restart.
- Chưa có collision giữa character, obstacle hoặc blocked strategy cell.
- `impact_ratio` hiện là dữ liệu tạm, chưa hiệu chỉnh theo animation Unity cuối cùng.
