# Movement, World Và Spatial Grid

## Movement Input

Unity gửi `MovementInput` qua opcode `1`:

```protobuf
message MovementInput {
  double x = 1;
  double y = 2;
  uint64 sequence = 3;
}
```

- `sequence` phải tăng theo session; packet cũ hoặc trùng bị bỏ qua.
- Vector dài hơn 1 được normalize để di chuyển chéo không nhanh hơn.
- Input `(0,0)` dừng movement nhưng giữ facing gần nhất.
- Nếu không nhận input mới trong `3 ticks` (`300 ms`), server đặt direction về zero.
- Giá trị NaN/Infinity và malformed protobuf bị từ chối.

## Movement Speed

Player không có speed riêng. Tốc độ tâm formation bằng `MoveSpeed` nhỏ nhất trong các character còn sống. Player không còn character có speed `0` và không thể dịch chuyển, dù direction/facing vẫn có thể được cập nhật.

Mỗi tick:

```text
distance = move_speed / 10
```

Ví dụ speed `5` đi `0.5 world unit/tick`.

## World

| Constant | Giá trị |
| --- | --- |
| `PlayAreaRadius` | `1000` |
| `SpawnRadius` | `990` |

World là hình tròn tâm `(0,0)`. Player và character target/position được clamp vào boundary. Spawn dùng phân phối đều theo diện tích hình tròn, không dồn điểm vào tâm.

## Spatial Grid Và Detection

Spatial grid dùng cell vuông `20 world units`, map cell bằng `floor(position / cellSize)` nên hỗ trợ tọa độ âm. Grid index player theo `SessionID` và hỗ trợ insert, move, remove.

Mỗi player có `DetectionRadius = 10`. Query quét cell ứng viên rồi lọc chính xác bằng khoảng cách Euclid hình tròn:

```text
dx² + dy² <= radius²
```

Kết quả loại chính player, bao gồm boundary và sắp theo khoảng cách rồi `SessionID`. Grid chỉ index tâm player, chưa index từng character hoặc projectile.

## Detection Snapshot

Opcode `102` gửi riêng mỗi player ở `10 Hz`, `reliable=false`:

- `self`: authoritative state của chính player.
- `players`: player khác trong detection radius.
- `projectiles`: projectile có attacker hoặc target thuộc nhóm player đang nhìn thấy.

Snapshot rỗng vẫn được gửi để Unity loại entity đã rời vùng detection.
