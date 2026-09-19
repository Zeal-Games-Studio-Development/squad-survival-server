# Combat Và Projectile

## Attack Eligibility

Combat tự động, không có attack input riêng. Character chỉ attack khi:

- Còn sống và có character ID hợp lệ.
- Weapon config có attack timing hợp lệ.
- Có character của player khác trong `AttackRange` hình tròn.
- `abs(Player.Direction.X) < 0.2` và `abs(Player.Direction.Y) < 0.2`.

Nếu một trục đạt `0.2` trở lên, movement được ưu tiên và attack cycle hiện tại bị reset. Projectile đã spawn vẫn tiếp tục tồn tại độc lập.

Target gần nhất được chọn; hòa khoảng cách thì theo target `UserID`, sau đó `Character.ID`. Target rời attack range hoặc chết trước impact làm đòn bị hủy.

## Attack Timing

```text
cycle_ticks  = ceil(10 / attack_speed)
impact_tick  = start_tick + ceil(cycle_ticks * impact_ratio)
complete_tick = start_tick + cycle_ticks
```

`AttackStarted` cung cấp cả ba tick để Unity scale animation. `AttackSequence` không reset khi hủy đòn, nên `AttackID = <character_id>:<sequence>` không bị tái sử dụng.

## Damage

Damage được roll tại server:

```text
spread = damage * (damage_ratio - 1)
result thuộc [max(0, damage - spread), damage + spread]
```

Kết quả hiện là `float64`, không làm tròn. Damage intent cùng tick được thu thập trước rồi áp theo thứ tự deterministic, nên hai character có thể hạ nhau trong cùng tick.

## Melee

Melee gây damage trực tiếp tại `impact_tick`:

```text
AttackStarted -> DamageApplied -> CharacterDied (nếu health về 0)
```

Spear thuộc melee và không tạo projectile.

## Ranged Projectile

Ranged spawn projectile tại `impact_tick`; projectile bắt đầu di chuyển từ tick kế tiếp. `combat.Simulation` giữ map projectile riêng cho từng match.

Projectile hiện là homing:

```text
direction = normalize(target.position - projectile.position)
position += direction * projectile_speed / 10
```

Khi khoảng cách còn lại nhỏ hơn quãng đường của một tick, projectile snap tới target, phát `ProjectileHit` và gây damage. Nếu target chết hoặc mất trước khi chạm, server phát `ProjectileExpired`.

## Events Và State

Opcode `103` phát reliable events: `AttackStarted`, `ProjectileSpawned`, `ProjectileHit`, `ProjectileExpired`, `DamageApplied`, `CharacterDied`.

Opcode `102` đồng thời chứa projectile đang active để Unity reconcile visual ngay cả khi không còn event spawn trong tick hiện tại.

## Giới Hạn Hiện Tại

- Projectile chưa bay theo đường thẳng cố định và target chưa thể né bằng displacement thông thường.
- Chưa có segment-circle collision, hit radius, collision với character khác hoặc terrain.
- Chưa có projectile lifetime/max distance.
- Chưa có armor, critical hit, AoE, line-of-sight hoặc regen processing.
