# Matchmaking Và Match Lifecycle

## Entry Points

Server hỗ trợ hai đường vào cùng coordinator:

- RPC `find_or_create_match` cho client/backend chủ động tìm trận.
- `RegisterMatchmakerMatched` khi Nakama matchmaker ghép được một nhóm entry.

RPC nhận JSON:

```json
{
  "mode": "survival"
}
```

Và trả:

```json
{
  "match_id": "...",
  "created": false,
  "already_joined": false
}
```

Nếu user đã join một authoritative match, RPC trả lại chính `match_id` đó với
`already_joined=true` thay vì tìm hoặc tạo match khác.

`mode` mặc định là `survival` và chỉ nhận 1-32 ký tự chữ, số, `_` hoặc `-`.

Server chỉ tìm/tạo match khi tài khoản có `squad_loadout` hợp lệ với đủ tám weapon type. Callback matchmaker kiểm tra từng thành viên trước khi trả match ID; `MatchJoinAttempt` và lúc tạo player kiểm tra lại để chặn join trực tiếp hoặc dữ liệu đã thay đổi. RPC vẫn trả match đang tham gia với `already_joined=true` mà không tìm match mới.

## Find Or Create

Coordinator tìm tối đa 20 authoritative match có label phù hợp, còn đủ slot và `joinable=true`. Survival có thể backfill trong `waiting` và `playing`; coordinator kiểm tra `available_slots` để nhóm ghép trận không vượt sức chứa của phase. Nhóm trên 3 người chỉ có thể vào trận Survival đang `playing`; nếu không có trận đủ chỗ, yêu cầu bị từ chối. Battle Royale chỉ backfill trong `waiting`. Danh sách được sắp theo size giảm dần để backfill match đông nhất trước.

Nếu không tìm thấy, server gọi `MatchCreate`. Mutex process-local bảo vệ đoạn find-or-create khỏi tạo trùng trong cùng Nakama process.

## Capacity

| Giá trị | Hiện tại |
| --- | --- |
| Max players | `32` |
| Survival waiting limit | `3` người, tính cả reservation |
| Reservation TTL | `10 giây` |
| Empty match TTL | `60 giây` khi `waiting` chưa từng có người; `15 giây` khi `playing` trống |
| Match list limit | `20` |

Capacity được tính bằng player đã join cộng reservation chưa hết hạn. Party lớn hơn capacity của phase hiện tại bị từ chối.

## Survival lifecycle

| Phase | Thời lượng | Join | Gameplay |
| --- | --- | --- | --- |
| `waiting` | Tối đa 30 giây từ người đầu tiên | Tối đa 3 người | Tạm dừng |
| `playing` | 15 phút | Tiếp tục nhận đến 32 người | Hoạt động |
| `ended` | 1 phút trước khi đóng match | Không | Tạm dừng |

Đủ 3 người đã join thì chuyển sang `playing` ngay; nếu chưa đủ, chuyển khi hết 30 giây. Khi `playing` hết 15 phút, server hủy các lượt nhặt hộp đang chờ và chuyển sang `ended`. Tổng kết điểm và xếp hạng chưa được triển khai. Lobby chưa từng có người đóng sau 60 giây trống; lobby đã bắt đầu countdown đóng ngay khi không còn người hoặc reservation. Trận `playing` trống đóng sau 15 giây; `ended` giữ đủ 1 phút dù không còn người.

## Active Match Registry

`modules/game/matchregistry` giữ membership process-local theo `UserID`, `SessionID`
và `MatchID`. Membership chỉ được thêm sau `MatchJoin` thành công, không phải khi
RPC mới trả về một match. Nhiều session của cùng user được phép vào cùng một match;
join sang match khác bị từ chối.

`MatchLeave` xóa session, còn `MatchTerminate` và empty-match shutdown dọn toàn bộ
membership của match. Registry nằm trong memory và được tạo một lần tại `InitModule`.

## Match Label

Label JSON gồm:

```json
{
  "mode": "survival",
  "status": "waiting",
  "player_count": 1,
  "max_players": 32,
  "available_slots": 2,
  "joinable": true
}
```

`status` đổi theo phase. Với Survival, `available_slots` tính từ giới hạn của phase trừ số người đã join và reservation còn hạn; `joinable` đúng khi còn chỗ trong `waiting` hoặc `playing`. Label được cập nhật khi phase đổi, reservation hết hạn, player join hoặc leave. Opcode `101` gửi lại player count khi join/leave; opcode `107` gửi phase và deadline cho người mới join và khi phase đổi.

## Giới Hạn Hiện Tại

- `MatchLeave` xóa player state ngay; chưa có reconnect grace period.
- Mutex không điều phối find-or-create giữa nhiều Nakama node.
- Match đang chạy không được migrate hoặc restore sau restart/deploy.
- Chưa có tổng kết điểm, xếp hạng hoặc reward settlement.
