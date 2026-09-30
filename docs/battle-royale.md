# Battle Royale Lifecycle

Mode `battle-royale` dùng cùng tick rate, giới hạn 32 player và gameplay hiện tại của Survival, bao gồm movement, spatial grid, combat, projectile, character và character box. Khác biệt chính là lifecycle có ba phase rõ ràng.

## Phase

| Phase | Thời lượng | Join | Gameplay |
| --- | --- | --- | --- |
| `waiting` | Tối đa 60 giây từ player đầu tiên | Có, đến khi đủ 32 player | Tạm dừng |
| `playing` | 10 phút | Không | Hoạt động |
| `ended` | 10 giây | Không | Tạm dừng |

Match bắt đầu `waiting`. Countdown chỉ bắt đầu khi player đầu tiên join thành công. Match chuyển sang `playing` khi countdown hết hoặc phòng đủ 32 player. Lobby đã từng có player sẽ terminate khi không còn player hoặc reservation; lobby chưa từng có player tiếp tục dùng empty TTL 60 giây.

Khi vào `playing`, server khóa join, đặt lại lịch refill character box theo thời điểm bắt đầu gameplay và chạy simulation giống Survival. Sau 10 phút, match chuyển sang `ended`, dừng input và simulation, rồi terminate sau 10 giây để client nhận trạng thái cuối.

## Matchmaking và label

RPC giữ nguyên:

```json
{
  "mode": "battle-royale"
}
```

Matchmaking chỉ backfill Battle Royale match có `status=waiting`, `joinable=true` và còn capacity. Label dùng `status` tương ứng với phase hiện tại; `joinable` luôn là `false` trong `playing` và `ended`.

## Realtime lifecycle

Opcode reliable `107` gửi protobuf `MatchLifecycleState` cho player khi join và broadcast khi phase thay đổi:

- `phase`: `MATCH_PHASE_WAITING`, `MATCH_PHASE_PLAYING` hoặc `MATCH_PHASE_ENDED`.
- `server_tick`: tick tại thời điểm gửi.
- `phase_ends_at_tick`: deadline hiện tại; bằng `0` khi lobby chưa có player đầu tiên.
- `tick_rate`: số tick mỗi giây để client tính countdown.

## Chưa triển khai

Vòng bo chưa được triển khai. Sau này vòng bo sẽ thu hẹp vùng chơi trong phase `playing` và bổ sung điều kiện chuyển sang `ended` bên cạnh timeout 10 phút, ví dụ khi chỉ còn một player hoặc đội sống sót.
