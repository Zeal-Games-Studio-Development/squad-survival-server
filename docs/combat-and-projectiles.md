# Combat và projectile

## Luồng đánh thường

Match gọi combat mỗi tick. Character đứng yên, còn sống và có target trong `attack_range` sẽ tự động đánh. Khi bắt đầu action, combat khóa tối đa `attack_count` enemy character gần nhất, không lặp target; hòa khoảng cách thì theo `UserID`, tiếp theo `Character.ID`. Nếu không đủ target, action chỉ có số lượt tương ứng với số target đã khóa. Player di chuyển (một trục input đạt `0.2`) sẽ hủy các lượt chưa phát. Nếu một target đã khóa chết, rời tầm hoặc không còn trong detection trước lượt của nó, chỉ lượt đó bị bỏ; các target khác vẫn giữ lượt. Không chọn target thay thế. Projectile đã sinh vẫn tiếp tục bay.

Target acquisition dùng các player trong detection radius từ spatial grid. Config yêu cầu `attack_range + query_buffer <= detection_radius`. Attack cycle: `cycle_ticks = ceil(10 / attack_speed)`, `impact_tick = start_tick + ceil(cycle_ticks * impact_ratio)`. `complete_tick` là mốc muộn hơn giữa cuối cycle và hit cuối của action.

Mỗi đòn đánh thường có một `action_id` duy nhất và tag `basic_attack`. `AttackStarted.attack_count` là số target thực tế đã khóa; danh sách `targets` giữ thứ tự lượt và hướng khóa lúc bắt đầu. Lượt đầu xảy ra tại `impact_tick`, các lượt sau cách nhau **2 tick**. Melee gây một hit và ranged sinh một projectile cho target của mỗi lượt; từng hit roll chí mạng độc lập. Ranged action thêm tag `projectile`. Spear thêm tag `aoe`: mỗi lượt tạo một vùng đánh theo hướng đã khóa cho target tương ứng, bắt đầu từ vị trí attacker lúc phát, dài bằng `attack_range`, bán kính va chạm `0,6`. Mỗi enemy trong vùng nhận một hit bằng damage của đòn thường; các vùng chồng nhau có thể đánh cùng enemy nhiều lần. Spear không còn nhân damage theo `attack_count`.

## Modifier theo weapon type

| Type | Quy tắc |
| --- | --- |
| Sword, shield | Mỗi lượt đánh một target cận chiến. Shield còn giảm 40% damage từ action có tag `projectile`, nhân sau giảm damage chung. |
| Axe | Mỗi lượt đánh một target; tăng 35% damage khi mục tiêu là `ranged`. |
| Blunt | Mỗi lượt đánh một target; tăng 35% damage khi mục tiêu là `melee`. |
| Bow | Damage projectile nhân từ `1×` đến `2×` theo khoảng cách giữa vị trí bắn và vị trí mục tiêu lúc chạm, chia cho `attack_range` đã ghi lúc bắn. |
| Crossbow | Crit chance cộng từ 0 đến 35 điểm phần trăm theo phần máu đã mất của mục tiêu; đạt mức tối đa khi mục tiêu còn 50% HP trở xuống. Tính trước từng hit lúc projectile chạm, không cộng vào stat của attacker. |
| Staff | `cooldown_scale = 1,25` cho chính character. |
| Spear | Đánh AoE theo đường thẳng như trên; không sinh projectile. |

Damage trước giảm chung bằng damage gốc nhân modifier theo type và crit multiplier nếu chí mạng. Giảm chung giới hạn ở 60%; riêng shield nhân thêm `0,6` sau bước này nếu hit mang tag `projectile`. `DamageApplied.damage` là lượng HP thực tế mất, không vượt quá HP còn lại. Các hit trong tick được sắp theo thứ tự ổn định trước khi áp dụng.

## Projectile, cooldown và protocol

Projectile xuất hiện tại impact tick, bắt đầu di chuyển từ tick sau và homing theo target. Khi chạm, nó phát `ProjectileHit` và `DamageApplied`; nếu target biến mất hoặc chết trước đó, phát `ProjectileExpired`. Vị trí projectile đang bay được gửi trong opcode `105`.

Skill theo `weapon_id` được khai báo trong [`skills.json`](../modules/game/core/entity/skills.json). `skill_id` dùng để định danh, tra cứu skill và theo dõi cooldown; luồng xử lý không rẽ nhánh theo ID mà theo cơ chế được khai báo trong skill (cách chọn mục tiêu, hình dạng vùng đánh, thời điểm impact và loại hiệu ứng). Hiện tại chỉ hỗ trợ hình dạng `single`/`cone` và hiệu ứng `damage`; cơ chế mới cần bổ sung handler trong combat. Cooldown có ba mode: mỗi tick, mỗi action đánh thường bắt đầu, hoặc mỗi action gây mất HP cho character. Nhiều hit của cùng một action chỉ tăng tiến độ một lần; mỗi lần tăng được nhân `cooldown_scale`. Character mới có tiến độ 0; khi skill bắt đầu, tiến độ trở về 0. Skill sẵn sàng mà chưa có mục tiêu thì không tiêu cooldown.

Một `weapon_id` có thể gắn với nhiều skill khác `skill_id`; mỗi skill có cooldown riêng. Catalog yêu cầu `skill_id` duy nhất trên toàn bộ skill và không cho lặp cùng `weapon_id` trong một định nghĩa skill, nhưng không giới hạn số skill của một weapon. Catalog nhúng được validate lúc `InitModule`; cấu hình sai khiến khởi tạo panic, không phải lỗi build. Khi thêm skill mới, cần cập nhật các test hiện đang giả định catalog chỉ có một skill.

Human chủ động kích hoạt skill bằng opcode `2` gồm `character_id` và `skill_id`; bot tự kích hoạt skill đủ điều kiện theo priority giảm dần rồi `skill_id`. Skill có thể bấm khi character còn sống, không đang thi triển skill khác, cooldown đầy và có target hợp lệ trong tầm. Đòn thường đang chạy hoặc player đang di chuyển không chặn skill: khi kích hoạt, các hit thường chưa phát bị hủy, projectile đã sinh tiếp tục bay. Server chọn target gần nhất, kiểm tra lại điều kiện lúc nhận lệnh và chỉ reset cooldown khi bắt đầu skill. Target selector phân biệt chính character (`self`), character cùng player (`ally`) và character của player khác (`enemy`). Skill mẫu `sword_cone` khóa hướng theo địch gần nhất, sau 2 tick đánh các địch trong nón 90 độ và tầm `attack_range` tại vị trí hiện tại của chúng. Mỗi địch nhận một hit bằng damage gốc nhân `1,2`, roll crit riêng; skill hoàn thành sau 5 tick và hồi sau 30 tick. Action có tags `skill`, `aoe`; `attack_count` và modifier riêng của đòn thường không áp dụng. Character đứng yên khi thi triển dù player di chuyển hoặc target ban đầu biến mất, rồi trở về slot theo tốc độ formation cũ. Nếu character chết trước impact, không có hit.

Opcode `103` gửi combat events reliable. `AttackStarted` chứa `action_id`, `attack_count` thực tế, danh sách `targets` theo thứ tự lượt với hướng khóa, `tags` và ba mốc tick; các trường target đơn giữ target đầu. `SkillStarted` chứa `skill_id`, `action_id`, hướng và ba mốc tick. Damage, death và projectile events chứa `action_id`, `hit_id`, `tags` để client ghép các hit thuộc cùng action. Opcode `110` gửi trạng thái cooldown, thi triển và danh sách `ready_skill_ids` khi thay đổi, giới hạn theo spatial detection. Danh sách ready rỗng có nghĩa là xóa trạng thái ready trước đó. Projectile vẫn chưa va chạm địa hình hoặc character khác trên đường bay.
