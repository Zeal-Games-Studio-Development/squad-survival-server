# Developer Handbook

Tài liệu này mô tả trạng thái đã triển khai của Squad Survival Server. Source code và các file `.proto` vẫn là nguồn sự thật cuối cùng khi có khác biệt.

## Technology

| Thành phần | Phiên bản |
| --- | --- |
| Nakama | `3.40.0` |
| Go | `1.26.5` |
| PostgreSQL | `16.8-alpine` |
| Protobuf Go runtime | `1.36.11` |
| Server tick rate | `10 Hz` |

## Implemented

| Nhóm | Trạng thái |
| --- | --- |
| Docker Compose và PostgreSQL persistence | Hoàn thành |
| Matchmaking, find-or-create và backfill | Hoàn thành |
| Authoritative movement và world boundary | Hoàn thành |
| Spatial detection theo vùng tròn | Hoàn thành |
| Character, weapon và formation 5x5 | Hoàn thành |
| Melee combat, ranged projectile và combat events | Hoàn thành |
| Realtime protocol bằng Protobuf | Hoàn thành |
| Generate Go và C# từ cùng schema | Hoàn thành |

## Mục Lục

1. [Cài đặt và chạy local](getting-started.md)
2. [Kiến trúc runtime](architecture.md)
3. [Matchmaking và match lifecycle](matchmaking.md)
4. [Movement, world và spatial grid](movement-and-world.md)
5. [Character và formation](characters-and-formations.md)
6. [Combat và projectile](combat-and-projectiles.md)
7. [Realtime protocol và Unity](realtime-protocol.md)
8. [Kiểm thử và code generation](testing.md)

## Planned

Các tính năng chưa hoàn thành không được mô tả như contract hiện tại. Xem [TODO.md](../TODO.md) để theo dõi reconnect state, strategy input và việc hiệu chỉnh `impact_ratio` theo animation Unity.
