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
| `PlayAreaRadius` | `500` |
| `SpawnRadius` | `450` |

World là hình tròn tâm `(0,0)`. Player và character target/position được clamp vào boundary. Spawn dùng phân phối đều theo diện tích hình tròn, không dồn điểm vào tâm.

## Spatial Grid Và Detection

Spatial grid dùng cell vuông `20 world units`, map cell bằng `floor(position / cellSize)` nên hỗ trợ tọa độ âm. Grid index player theo `SessionID` và hỗ trợ insert, move, remove.

Mỗi player có `DetectionRadius = 20`. Query quét cell ứng viên rồi lọc chính xác bằng khoảng cách Euclid hình tròn:

```text
dx² + dy² <= radius²
```

Kết quả loại chính player, bao gồm boundary và sắp theo khoảng cách rồi `SessionID`. Grid chỉ index tâm player, chưa index từng character hoặc projectile.

## Movement Snapshots

Opcode `102` gửi riêng mỗi player ở `10 Hz`, `reliable=false`:

- `self`: authoritative state của chính player.
- `players`: player khác trong detection radius.
- Projectile correction được gửi riêng bằng opcode `105`, chỉ gồm projectile ID và position.

Snapshot rỗng vẫn được gửi để Unity loại entity đã rời vùng detection.

Client xem danh sách `players` của snapshot mới nhất là tập player đang được render trong detection. Player leave match hoặc đi ra khỏi detection sẽ biến mất khỏi danh sách này; server không gửi thêm `PlayerRosterBatch` removal. Roster chỉ cung cấp static metadata, character loadout và skin khi entity xuất hiện hoặc roster thay đổi.

## Player Detection (Snapshot) So Với Experience Package (Delta)

Cùng dùng spatial grid và cùng ngưỡng `DetectionRadius` để xác định thứ gì nằm trong tầm player, nhưng hai loại entity chọn cơ chế đồng bộ khác nhau vì tính chất state khác nhau:

| Đặc điểm | Player detection | Experience package |
| --- | --- | --- |
| Opcode | `102` `PlayerMovementSnapshot` | `109` `ExperiencePackageStateBatch` |
| Cơ chế | Snapshot toàn bộ mỗi tick | Delta theo từng event |
| Reliability | `reliable=false` | `reliable=true` |
| Tần suất | Mỗi tick (`10 Hz`) | Chỉ khi trạng thái đổi |
| Tín hiệu rời tầm | Vắng mặt trong snapshot kế tiếp | Event `LOST` tường minh |
| Tín hiệu biến mất khác | Không có (chỉ rời tầm) | Event `COLLECTED` (kèm collector) |

Player là state biến đổi liên tục (position/character mỗi tick), nên snapshot mỗi tick là cách rẻ và đơn giản nhất để client reconcile: danh sách mới nhất chính là tập đang render, `session_id` vắng mặt đồng nghĩa đã rời detection.

Experience package là object tĩnh cho tới khi bị nhặt. Mỗi observer chỉ thấy vài gói trong `DetectionRadius` và các lần chuyển trạng thái (vào tầm, rời tầm, bị nhặt) rất thưa, nên gửi delta khi có event rẻ hơn nhiều so với snapshot `10 Hz`. Vì **không có snapshot mỗi tick**, client không thể suy ra "vắng mặt = rời tầm"; do đó server phải phát tín hiệu tường minh: `DETECTED` khi vào tầm, `LOST` khi rời tầm, `COLLECTED` khi bị nhặt. Reliability `true` bảo đảm client không lỡ các event xoá object này.

Hệ quả thiết kế: không thể bỏ `LOST`/`COLLECTED` hoặc gộp chúng thành "vắng mặt" như với player, trừ khi đổi hẳn sang mô hình snapshot cho package. Tập `Known` per-session trong `experience.Manager` chính là state client-side được server giữ hộ để tính delta.
