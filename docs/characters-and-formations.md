# Character Và Formation

## Level, Experience Và Giới Hạn Character

Mỗi player trong match có progression runtime, khởi đầu ở level `1` với experience `0`. Progression chưa được lưu vào storage và chưa có nguồn nhận experience.

Giới hạn character hiện tại được tính bằng `min(level, 9)`: level 1 có 1 slot, level 2 có 2 slot và từ level 9 trở lên có 9 slot. Hằng `strategy.MaxCharacters = 9` là giới hạn tuyệt đối của formation. Bảng experience nằm trong `modules/game/core/progression/level_progression.json`; mỗi giá trị là experience cần thêm để sang level kế tiếp.

## Character Lifecycle

Người chơi thật vào trận với một character được chọn ngẫu nhiên đều từ các `weapon_id` common đang sở hữu. Bot dùng inventory giả lập gồm toàn bộ weapon trong catalog, bao gồm common, uncommon và rare; starter vẫn chỉ chọn common. Character ID có dạng `<user_id>:<sequence>` và ổn định khi array được compact.

Character có position, target position, health/max health, damage, movement/attack stats, weapon và combat state. Khi `Health <= 0`, character bị remove trong combat tick; formation được gán lại trước snapshot tiếp theo.

Mỗi player tối đa `9` character non-nil. `nil` không chiếm slot và không xuất hiện trong snapshot.

## Weapon Catalog

Catalog mặc định nằm tại [`weapons.json`](../modules/game/core/entity/weapons.json). Weapon thay thế toàn bộ gameplay stats của character khi equip.

| Field | Ý nghĩa |
| --- | --- |
| `type` | Loại weapon gameplay |
| `weapon_id` | ID duy nhất, ổn định để inventory và Unity đối chiếu asset/reference |
| `rarity` | Bắt buộc: `common`, `uncommon` hoặc `rare` |
| `range_class` | `melee` hoặc `ranged` |
| `health` | Max health sau khi equip |
| `damage` | Sát thương gốc cố định của mỗi đòn |
| `crit_chance`, `crit_multiplier` | Xác suất chí mạng và hệ số nhân sát thương gốc |
| `damage_reduction` | Tỉ lệ giảm sát thương khi nhận đòn, tối đa 60% |
| `move_speed` | World units mỗi giây |
| `attack_speed` | Số attack cycle mỗi giây |
| `attack_range` | Bán kính khóa target |
| `attack_count` | Số enemy character tối đa được khóa cho một đòn thường; mỗi target có một lượt melee, projectile ranged hoặc AoE spear |
| `cooldown_scale` | Hệ số tăng tiến độ hồi cooldown theo tick, action đánh và action nhận damage |
| `impact_ratio` | Vị trí impact trong attack cycle |
| `projectile_speed` | World units mỗi giây; ranged phải lớn hơn 0 |
| `regen_rate` | Stat đã có, chưa có regen system |

Catalog gồm 24 weapon: tám ID cơ bản (`archer`, `mage`, `farmer`, `swordman`, `axeman`, `bruiser`, `hunter`, `shieldbearer`) là common; mỗi ID có thêm biến thể `<id>_uncommon` và `<id>_rare`. Ví dụ: `archer`, `archer_uncommon`, `archer_rare`. Mỗi rarity có tám weapon. Common giữ nguyên chỉ số; uncommon kế thừa bộ chỉ số swift cũ, rare kế thừa bộ chỉ số heavy cũ.

`spear`, `sword`, `axe`, `blunt`, `shield` là melee. `bow`, `staff`, `crossbow` là ranged.

## Character Inventory Và Character Khởi Đầu

Nakama Storage lưu object `character_inventory/inventory` cho mỗi tài khoản mới, chỉ gồm `weapon_ids`. Object có `permission_read=1` (chủ tài khoản được đọc) và `permission_write=0` (chỉ server được ghi). Config khởi tạo là [`initial_character_inventory.json`](../modules/game/inventory/initial_character_inventory.json); chỉ số của từng weapon lấy từ `weapons.json`. Chỉ tài khoản mới được cấp danh sách này; tài khoản cũ không được migration hoặc cập nhật lại inventory.

Sau khi đăng nhập, client đọc Nakama Storage object với collection `character_inventory`, key `inventory` và `user_id` của session, rồi lấy `weapon_ids` từ JSON value để hiển thị. Inventory phải tồn tại, có ít nhất một ID đã biết và không có ID trùng; không cần đủ tám type. Inventory không hợp lệ bị chặn khi tìm trận và khi join trực tiếp. Danh sách initial mặc định khai báo tường minh 24 ID: tám common, tám uncommon và tám rare. Server kiểm tra danh sách không rỗng, ID tồn tại trong catalog và không trùng; cho phép nhiều weapon cùng type khác rarity. Mở rộng catalog không tự động bổ sung ID vào config cấp ban đầu. Khi join trận, inventory phải có ít nhất một common hợp lệ. Server nạp lại inventory khi join thực tế và giữ snapshot theo session cho đến lúc rời trận; thay đổi Storage giữa trận không thay đổi snapshot. Config initial chỉ dùng để cấp inventory lúc tạo tài khoản, không dùng chọn starter hoặc character từ box.

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

Match duy trì Character Box tĩnh và mỗi phút refill đến target của mode. Box chứa `weapon_type` và `rarity` (`common`, `uncommon`, `rare`). Mỗi đợt sinh mới chia số lượng theo trọng số common : uncommon : rare = 7 : 2 : 1; ví dụ 60 box gồm 42 common, 12 uncommon và 6 rare. Khi refill, chỉ áp dụng tỉ lệ cho số box bổ sung, giữ nguyên box đang có. Với mỗi rarity, chọn đều giữa các type duy nhất mà ít nhất một người hoặc bot chưa bị loại sở hữu. Bot được tạo trước đợt spawn box đầu. Rarity có pool rỗng bị loại và chia lại theo trọng số còn lại (ví dụ common/uncommon là 7:2); mọi pool rỗng thì không spawn. Lấy phần nguyên rồi phân phối số dư theo phần thập phân lớn nhất, hòa thì ưu tiên common, uncommon, rare; 61 box là 43/12/6. Danh sách rarity được xáo trộn trước khi đặt vị trí; nếu hết chỗ thì giữ box đã đặt và cảnh báo thiếu số lượng. Box đã tồn tại được giữ khi thành viên thay đổi. Khi player đi vào bán kính collision, server khóa một box cho một player và bắt đầu countdown; mỗi player chỉ được claim một box. Player phải sở hữu ít nhất một weapon khớp type/rarity, ở trong vùng, còn sống và còn capacity theo level cho tới deadline. Người không đủ điều kiện bị bỏ qua khi chọn claimant; claim mất điều kiện sẽ bị cancel. Delay dựa trên số character lúc bắt đầu claim: count 1–4 chờ tương ứng 1–4 giây, 5–7 chờ 5 giây và 8 chờ 6 giây. Player đã đạt giới hạn runtime không thể claim; giới hạn tuyệt đối là 9 character. Bảng thời gian nằm trong `modules/game/core/characterbox/pickup_delays.json`.

Opcode reliable `106` broadcast `PICKUP_STARTED` và `PICKUP_CANCELLED`; client tự tính countdown từ deadline tick. Khi hoàn tất, server grant character và gửi `DESPAWNED`. Box đang được claim vẫn tồn tại trong world và được tính khi refill.

Khi hoàn tất nhặt box, server random đều một `weapon_id` trong snapshot inventory của người nhặt, khớp `weapon_type` và `rarity` của box. Character nhận từ box chỉ tồn tại trong trận, không mở khóa ID trong inventory tài khoản. Logic lọc/chọn được dùng chung cho Survival, Battle Royale và bot. Chưa triển khai ghép `2 common → 1 uncommon` hoặc `2 uncommon → 1 rare`.

## Giới Hạn Hiện Tại

- Client chưa có opcode chọn strategy; mọi player dùng mặc định `x-type`.
- Character chưa persist qua match restart.
- Chưa có collision giữa character, obstacle hoặc blocked strategy cell.
- `impact_ratio` hiện là dữ liệu tạm, chưa hiệu chỉnh theo animation Unity cuối cùng.
