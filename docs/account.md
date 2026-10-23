# Tài khoản và đổi tên hiển thị

Client đặt tên qua RPC `change_display_name`. API `UpdateAccount` vẫn chặn thay đổi profile trực tiếp.

Request và response payload của RPC đều có dạng:

```json
{"display_name":"Player One"}
```

- Tên hiện tại rỗng: miễn phí, kể cả khi ví không có gem.
- Đã có tên: đổi sang tên khác mất 500 gem. Tên được nhập từ Google/Facebook cũng tính là đã có tên.
- Tên gửi lên giống tên hiện tại sau chuẩn hóa NFC và bỏ khoảng trắng đầu/cuối: thành công, không tính phí.
- Tên mới vẫn phải dài 3–12 ký tự, vượt qua bộ lọc ký tự và từ ngữ hiện có.

Server cập nhật tên và trừ tiền trong cùng giao dịch `MultiUpdate`. Khi tính phí, wallet ledger có metadata `{"event":"change_display_name"}`. Các sự kiện economy dùng chung trường `event`; khi không truyền event riêng, thao tác chi tiền mặc định dùng `payment`, cấp tiền dùng `currency_grant`. Nếu giao dịch lỗi, tên, ví và storage guard đều được rollback.

Storage guard riêng từng user nằm ở collection `account_operations`, key `change_display_name`, quyền đọc/ghi client đều bằng 0. Conditional version write và nonce mới cho mỗi lần đổi tên ngăn hai yêu cầu đồng thời cùng dùng lượt miễn phí. Khi xung đột, server đọc lại trạng thái và thử lại, tối đa 3 lần thực hiện giao dịch.

| Mã lỗi RPC | Ý nghĩa |
| --- | --- |
| 3 | Payload hoặc tên không hợp lệ. |
| 9 | `insufficient gem`: không đủ gem; không thay đổi tên hoặc số dư. |
| 10 | `display name change conflicted; retry`: hết lượt thử lại do xung đột; client có thể gửi lại. |
| 13 | Lỗi hệ thống. |
| 16 | Chưa xác thực. |

## Cấu hình economy

`modules/economy/currency.json` định nghĩa currency và số dư khởi tạo. Chi phí tính năng nằm trong `modules/economy/economy_general.json`:

```json
{
  "costs": {
    "change_display_name": {"gem": 500}
  }
}
```

Server kiểm tra cấu hình khi khởi động: phải có chi phí đổi tên; mọi chi phí phải có currency hợp lệ và số nguyên dương. Có thể bổ sung các mục chi phí gacha vào `costs` khi triển khai tính năng tương ứng. Cấu hình được embed vào plugin nên thay giá cần build và triển khai lại. Không cần migration; guard được tạo khi đổi tên lần đầu.

## Kiểm thử tích hợp

`TestDisplayNameIntegration` chỉ chạy khi đặt cả `NAKAMA_RENAME_TEST_URL` và `NAKAMA_RENAME_TEST_POSTGRES_CONTAINER`. Dùng một stack Nakama/Postgres **riêng cho test**, với server key `defaultkey`, database `nakama`, user Postgres `postgres`, và Docker CLI trên máy chạy test. Test tạo user và thay số dư ví của những user đó bằng SQL, kiểm tra API, rollback, ledger, quyền storage và yêu cầu đồng thời.

```powershell
$env:NAKAMA_RENAME_TEST_URL = "http://127.0.0.1:<test-port>"
$env:NAKAMA_RENAME_TEST_POSTGRES_CONTAINER = "<test-postgres-container>"
go test -count=1 -run TestDisplayNameIntegration -v ./modules/account
```
