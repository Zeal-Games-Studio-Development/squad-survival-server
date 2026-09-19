# Matchmaking Và Match Lifecycle

## Entry Points

Server hỗ trợ hai đường vào cùng coordinator:

- RPC `find_or_create_match` cho client/backend chủ động tìm trận.
- `RegisterMatchmakerMatched` khi Nakama matchmaker ghép được một nhóm entry.

RPC nhận JSON:

```json
{
  "mode": "survival",
  "allow_join_in_progress": true
}
```

Và trả:

```json
{
  "match_id": "...",
  "created": false
}
```

`mode` mặc định là `survival` và chỉ nhận 1-32 ký tự chữ, số, `_` hoặc `-`.

## Find Or Create

Khi cho phép join in progress, coordinator tìm tối đa 20 authoritative match có label phù hợp, còn đủ slot và `joinable=true`. Danh sách được sắp theo size giảm dần để backfill match đông nhất trước.

Nếu không tìm thấy, server gọi `MatchCreate`. Mutex process-local bảo vệ đoạn find-or-create khỏi tạo trùng trong cùng Nakama process.

## Capacity

| Giá trị | Hiện tại |
| --- | --- |
| Max players | `32` |
| Reservation TTL | `10 giây` |
| Empty match TTL | `60 giây` |
| Match list limit | `20` |

Capacity được tính bằng player đã join cộng reservation chưa hết hạn. Party lớn hơn capacity bị từ chối.

## Match Label

Label JSON gồm:

```json
{
  "mode": "survival",
  "status": "playing",
  "player_count": 1,
  "max_players": 32,
  "joinable": true
}
```

Label được cập nhật khi reservation hết hạn, player join hoặc leave. Opcode `101` gửi lại player count khi join/leave.

## Giới Hạn Hiện Tại

- `MatchLeave` xóa player state ngay; chưa có reconnect grace period.
- Mutex không điều phối find-or-create giữa nhiều Nakama node.
- Match đang chạy không được migrate hoặc restore sau restart/deploy.
- Chưa có end-of-match phase hoặc reward settlement.
