# Combat và projectile

## Luồng đánh thường

Match gọi `combat.Step(...)` mỗi tick. Character đứng yên, còn sống và có target trong `attack_range` sẽ tự động đánh. Combat khóa mục tiêu gần nhất; hòa khoảng cách thì theo `UserID`, tiếp theo `Character.ID`. Khi player di chuyển (một trục input đạt `0.2`), mục tiêu chết hoặc rời tầm trước impact, action đang chờ bị hủy. Projectile đã sinh vẫn tiếp tục bay.

Target acquisition dùng các player trong detection radius từ spatial grid. Config yêu cầu `attack_range + query_buffer <= detection_radius`. Attack cycle: `cycle_ticks = ceil(10 / attack_speed)`, `impact_tick = start_tick + ceil(cycle_ticks * impact_ratio)`. `complete_tick` là mốc muộn hơn giữa cuối cycle và hit cuối của action.

Mỗi đòn đánh thường có một `action_id` duy nhất và tag `basic_attack`. `attack_count` là số hit cận chiến hoặc projectile ranged thuộc action đó; từng hit có `hit_id` và roll chí mạng độc lập. Hit đầu tiên xảy ra tại `impact_tick`, các hit tiếp theo cách nhau **2 tick**; projectile cũng được sinh theo lịch này. Nếu action bị hủy trước hit tiếp theo, các hit chưa phát không xảy ra, còn projectile đã sinh vẫn bay. Ranged action thêm tag `projectile`. Spear thêm tag `aoe`: đánh các đối thủ trên đoạn thẳng hướng tới mục tiêu đã khóa, dài bằng `attack_range`, bán kính va chạm `0,6`. Spear chỉ nổ một lần tại `impact_tick`, không giãn hit theo `attack_count`; mỗi mục tiêu trên đường nhận một hit. Khi `attack_count > 1`, hit AoE gây `damage × 1,5 × attack_count`; khi bằng 1, gây damage gốc.

## Modifier theo weapon type

| Type | Quy tắc |
| --- | --- |
| Sword, shield | Đánh một mục tiêu cận chiến. Shield còn giảm 40% damage từ action có tag `projectile`, nhân sau giảm damage chung. |
| Axe | Đánh một mục tiêu; tăng 35% damage khi mục tiêu là `ranged`. |
| Blunt | Đánh một mục tiêu; tăng 35% damage khi mục tiêu là `melee`. |
| Bow | Damage projectile nhân từ `1×` đến `2×` theo khoảng cách giữa vị trí bắn và vị trí mục tiêu lúc chạm, chia cho `attack_range` đã ghi lúc bắn. |
| Crossbow | Crit chance cộng từ 0 đến 35 điểm phần trăm theo phần máu đã mất của mục tiêu; đạt mức tối đa khi mục tiêu còn 50% HP trở xuống. Tính trước từng hit lúc projectile chạm, không cộng vào stat của attacker. |
| Staff | `cooldown_scale = 1,25` cho chính character. |
| Wand | Giữ đòn bắn một mục tiêu. |
| Spear | Đánh AoE theo đường thẳng như trên; không sinh projectile. |

Damage trước giảm chung bằng damage gốc nhân modifier theo type và crit multiplier nếu chí mạng. Giảm chung giới hạn ở 60%; riêng shield nhân thêm `0,6` sau bước này nếu hit mang tag `projectile`. `DamageApplied.damage` là lượng HP thực tế mất, không vượt quá HP còn lại. Các hit trong tick được sắp theo thứ tự ổn định trước khi áp dụng.

## Projectile, cooldown và protocol

Projectile xuất hiện tại impact tick, bắt đầu di chuyển từ tick sau và homing theo target. Khi chạm, nó phát `ProjectileHit` và `DamageApplied`; nếu target biến mất hoặc chết trước đó, phát `ProjectileExpired`. Vị trí projectile đang bay được gửi trong opcode `105`.

Cooldown cho skill `weapon_id` có ba mode: mỗi tick, mỗi action đánh thường bắt đầu, hoặc mỗi action gây mất HP cho character. Nhiều hit của cùng một action chỉ tăng tiến độ một lần; mỗi lần tăng được nhân `cooldown_scale`. Hiện chưa có skill `weapon_id` cụ thể, kích hoạt skill hoặc debuff. Bộ đếm nằm trong runtime của character.

Opcode `103` gửi combat events reliable. `AttackStarted` chứa `action_id`, `attack_count`, `tags` và ba mốc tick. Damage, death và projectile events chứa `action_id`, `hit_id`, `tags` để client ghép các hit thuộc cùng action. Schema đã đổi trực tiếp từ `attack_id` sang `action_id`; Unity cần dùng `Combat.cs` mới. Projectile vẫn chưa va chạm địa hình hoặc character khác trên đường bay.
