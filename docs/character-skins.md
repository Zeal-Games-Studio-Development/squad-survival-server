# Character Skin Runtime

## Inventory snapshot

Khi player join Survival hoặc Battle Royale, server batch-read object `player_inventory/skins` và cache một snapshot theo session trong state của match. Skin player quay thêm trong lúc trận đang chạy chỉ có hiệu lực ở trận sau. Cache được xóa khi session leave hoặc khi match state bị terminate.

Nếu storage lỗi, object không tồn tại, JSON invalid hoặc chứa item không có trong catalog, player vẫn được join và toàn bộ skin cần thiết fallback về numeric ID `1`.

## Random loadout

Mỗi character random độc lập từ các ID player sở hữu:

- Ngoại hình luôn gồm `hair`, `beard`, `chest`, `eye` và `helmet`.
- `weapon_id` lấy từ part trùng với `weapon_type` của character.
- Bow dùng `projectile_id` từ part `arrow`.
- Dagger chưa có part trong catalog nên dùng `weapon_id=0`; client render default dagger asset.
- Part bị thiếu trong inventory dùng ID `1`. Exclusive item vẫn được chọn nếu có trong inventory.

Skin được gán sau weapon và trước khi character được thêm vào roster. Skin chỉ phục vụ render, không thay đổi health, damage, movement hoặc combat.

Match dùng một cosmetic RNG riêng cho skin. RNG gameplay dành cho spawn, weapon, combat và damage không bị dịch chuyển khi server random skin hoặc khi `AddCharacter` thất bại.

## Realtime contract

`CharacterRoster` có field additive `skin` kiểu `CharacterSkin`:

```protobuf
message CharacterSkin {
  int32 hair_id = 1;
  int32 beard_id = 2;
  int32 chest_id = 3;
  int32 eye_id = 4;
  int32 helmet_id = 5;
  int32 weapon_id = 6;
  int32 projectile_id = 7;
}
```

Client tiếp tục dùng `weapon_type` trong roster để xác định loại asset và dùng `weapon_id` làm variant numeric ID.
