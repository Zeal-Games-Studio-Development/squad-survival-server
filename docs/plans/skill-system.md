# Skill theo weapon ID

## Mục tiêu

Combat tự động kích hoạt skill của `weapon_id` khi character rảnh, skill đã hồi và có mục tiêu hợp lệ. Skill dùng action riêng với tag `skill`; nhiều skill có thể thuộc cùng một weapon ID. Skill mẫu đầu tiên là sword quét hình nón. Cùng một luồng chạy cho Survival và Battle Royale.

## Catalog và runtime

- `skills.json` là catalog riêng, ánh xạ `weapon_id` tới các skill có `skill_id`, priority, cooldown `{mode, required}`, timing `{impact_ticks, complete_ticks}`, target selector và effect. Validate catalog và tham chiếu weapon ID khi khởi tạo module. Các weapon ID khác chưa có skill.
- Target selector phân biệt `self`, `ally` (character cùng player) và `enemy` (player khác). Không có team ID giữa các player. Effect handler đầu tiên xử lý damage hình nón; buff và debuff cụ thể bổ sung sau.
- Character bắt đầu cooldown tại 0. Mỗi tick, action đánh thường bắt đầu hoặc action gây mất HP tăng tiến độ theo mode, nhân `cooldown_scale`; nhiều hit cùng action chỉ tính một lần. Bắt đầu skill mới tiêu cooldown và reset tiến độ về 0. Nếu chưa có mục tiêu hợp lệ, skill vẫn sẵn sàng.
- Khi rảnh, ưu tiên skill sẵn sàng theo priority giảm dần rồi `skill_id`, trước đòn thường. Skill không dùng `attack_count` hay modifier damage dành riêng cho đòn thường; có `action_id` và tag `skill`.

## Skill và thi triển

- Sword quét nón 90 độ, tầm bằng `attack_range`, damage bằng 100% stat damage trước giảm sát thương, crit độc lập trên từng mục tiêu, cooldown 30 tick, impact sau 2 tick và hoàn thành sau 5 tick. Cấu hình effect cho phép skill khác tắt crit.
- Hướng nón khóa theo địch gần nhất lúc bắt đầu. Khi impact, tìm địch theo vị trí hiện tại bằng spatial detection; mỗi mục tiêu trong nón nhận một hit. Mục tiêu ban đầu biến mất hoặc player di chuyển không hủy action. Character đang thi triển đứng yên đến hết action; sau đó trở về slot bằng tốc độ formation hiện có. Character chết trước impact không gây hit.
- Event bắt đầu skill mang `skill_id`, `action_id`, hướng, start/impact/complete tick. Damage mang tags `skill`, `aoe` và `hit_id` riêng.

## Client và kiểm thử

- Bổ sung `SkillStarted` vào combat protobuf, cooldown ban đầu vào roster và opcode riêng cho tiến độ cooldown/trạng thái cast. Client thấy character của bản thân và các player trong phạm vi detection; roster cung cấp trạng thái khi player đi vào phạm vi. Generate lại Go/C# và cập nhật tài liệu protocol.
- Kiểm thử parser, ưu tiên skill, cooldown ba mode, AoE và crit, lock movement, spatial visibility, protobuf, Survival và Battle Royale; chạy `go test ./...`.

## Giả định

Skill được server tự kích hoạt. Đợt này chỉ có effect damage hình nón. Buff, debuff và quy tắc cộng dồn sẽ được định nghĩa khi thêm skill tương ứng.
