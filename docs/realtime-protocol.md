# Realtime Protocol Và Unity

## Opcode

| Opcode | Hướng | Payload | Reliability | Tần suất |
| --- | --- | --- | --- | --- |
| `1` | Client → Server | `MovementInput` | Do client chọn | Theo input |
| `101` | Server → Client | `StateSnapshot` | Reliable | Join/leave |
| `102` | Server → Client | `PlayerMovementSnapshot` | Unreliable | Mỗi tick |
| `103` | Server → Client | `CombatEventBatch` | Reliable | Khi có event |
| `104` | Server → Client | `PlayerRosterBatch` | Reliable | Join/enter detection/roster changed |
| `105` | Server → Client | `ProjectileMovementSnapshot` | Unreliable | Mỗi tick |
| `106` | Server → Client | `CharacterBoxStateBatch` | Reliable | Box spawn/despawn |

Contract nằm trong các source schema:

- [`input.proto`](../modules/game/core/entity/input.proto)
- [`state.proto`](../modules/game/core/system/state.proto)
- [`player_movement.proto`](../modules/game/core/system/player_movement.proto)
- [`roster.proto`](../modules/game/core/system/roster.proto)
- [`projectile_movement.proto`](../modules/game/core/system/projectile_movement.proto)
- [`vector.proto`](../modules/game/core/system/vector.proto)
- [`combat.proto`](../modules/game/core/system/combat.proto)
- [`character_box.proto`](../modules/game/core/entity/character_box.proto)
- [`character_box_state.proto`](../modules/game/core/system/character_box_state.proto)

Không đổi hoặc tái sử dụng field number đã phát hành. Field bị xóa trong tương lai cần được đánh dấu `reserved`.

## Packet Flow

```mermaid
sequenceDiagram
    participant Unity
    participant NakamaSDK as Nakama SDK
    participant Match as Go Match
    Unity->>NakamaSDK: MovementInput.ToByteArray()
    NakamaSDK->>Match: opcode 1 + bytes
    Match->>Match: proto.Unmarshal + simulation
    Match->>NakamaSDK: opcode 101/102/103/104/105/106 + proto.Marshal bytes
    NakamaSDK->>Unity: ReceivedMatchState
    Unity->>Unity: Message.Parser.ParseFrom(state.State)
```

Nakama SDK vận chuyển `byte[]`; Google.Protobuf chuyển giữa bytes và generated C# object.

## Unity Send

```csharp
var input = new MovementInput {
    X = direction.x,
    Y = direction.y,
    Sequence = ++sequence
};

await socket.SendMatchStateAsync(match.Id, 1, input.ToByteArray());
```

## Unity Receive

```csharp
socket.ReceivedMatchState += state => {
    switch (state.OpCode) {
        case 101:
            Handle(StateSnapshot.Parser.ParseFrom(state.State));
            break;
        case 102:
            Handle(PlayerMovementSnapshot.Parser.ParseFrom(state.State));
            break;
        case 103:
            Handle(CombatEventBatch.Parser.ParseFrom(state.State));
            break;
        case 104:
            Handle(PlayerRosterBatch.Parser.ParseFrom(state.State));
            break;
        case 105:
            Handle(ProjectileMovementSnapshot.Parser.ParseFrom(state.State));
            break;
        case 106:
            Handle(CharacterBoxStateBatch.Parser.ParseFrom(state.State));
            break;
    }
};
```

`CombatEvent` dùng protobuf `oneof`; Unity kiểm tra `EventCase` trước khi đọc `AttackStarted`, projectile event, damage hoặc death.

## Generate C#

Project dùng Buf remote plugins:

```powershell
buf generate
```

Go `.pb.go` được giữ trong backend repo. C# được tạo local tại `clients/unity/Generated/Protobuf` và bị Git ignore vì Unity nằm ở repo riêng; copy `Input.cs`, `State.cs`, `PlayerMovement.cs`, `Roster.cs`, `ProjectileMovement.cs`, `Vector.cs`, `Combat.cs` sang Unity sau khi schema thay đổi.

Unity cần Nakama SDK và `Google.Protobuf` runtime, không cần cài Buf hoặc `protoc` nếu chỉ sử dụng generated `.cs`.

## JSON Còn Được Dùng Ở Đâu

- RPC `find_or_create_match` request/response.
- Match label cho `MatchList` query.
- Embedded `weapons.json` và `strategies.json`.
- RPC `healthcheck` response.

Realtime opcode `1`, `101`, `102`, `103`, `104`, `105`, `106` đều dùng Protobuf binary.
