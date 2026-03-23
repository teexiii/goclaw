# Lên Lịch Tự Động: Cron & Heartbeat

## Cron — Agent Chạy Tự Động

Agent không chỉ chờ bạn chat. Với cron, agent tự chạy theo lịch.

### 3 Kiểu Lịch

| Kiểu | Ví dụ | Dùng khi |
|---|---|---|
| **at** | `"2026-03-25T08:00:00+07:00"` | Chạy 1 lần vào thời điểm cụ thể |
| **every** | `"30m"`, `"2h"`, `"1d"` | Lặp lại đều đặn |
| **cron** | `"0 8 * * 1-5"` (8h sáng T2-T6) | Cron expression linh hoạt |

### Tạo Cron Job

Agent tự tạo qua tool `cron`:

```
User: "Mỗi sáng 8h check giá Bitcoin và báo cho tôi"

Agent → cron.create:
{
  "name": "check-btc-daily",
  "schedule": { "kind": "cron", "expr": "0 8 * * *", "tz": "Asia/Ho_Chi_Minh" },
  "payload": {
    "kind": "agent",
    "message": "Check giá Bitcoin hiện tại, so sánh với hôm qua. Nếu biến động > 5% thì message user ngay."
  }
}
```

Hoặc qua RPC:
```json
{
  "method": "cron.create",
  "params": {
    "name": "daily-report",
    "agent_id": "uuid",
    "schedule": { "kind": "cron", "expr": "0 9 * * 1", "tz": "Asia/Ho_Chi_Minh" },
    "payload": { "kind": "agent", "message": "Tổng hợp báo cáo tuần" }
  }
}
```

### Payload Types

| Kind | Hành vi |
|---|---|
| `agent` | Agent nhận message và chạy full loop (think→act→observe) |
| `deliver` | Gửi message cố định tới user qua kênh (Telegram, Discord...) |
| `command` | Chạy shell command |

### Ví Dụ Thực Tế

**Morning briefing:**
```
Cron: 0 7 * * 1-5 (7h sáng T2-T6)
Message: "Tổng hợp tin tức tech quan trọng hôm nay. Web search 5 nguồn.
          Gửi kết quả qua Telegram cho user."
```

**Weekly review:**
```
Cron: 0 17 * * 5 (5h chiều thứ 6)
Message: "Tổng hợp các task đã complete tuần này từ team board.
          Viết weekly summary. Gửi qua Slack channel #team-updates."
```

**Price alert:**
```
Cron: */15 * * * * (mỗi 15 phút)
Message: "Check giá VNINDEX. Nếu giảm > 2% trong 1h → message user ngay."
```

**One-time reminder:**
```
Schedule: at 2026-03-25T14:00:00+07:00
Message: "Nhắc user: cuộc họp với đối tác lúc 3h chiều hôm nay."
Delete after run: true
```

## Heartbeat — Giám Sát Liên Tục

Heartbeat là cron đặc biệt: agent chạy checklist định kỳ và chỉ alert khi có vấn đề.

### HEARTBEAT.md

```markdown
# Monitoring Checklist

## Website
- [ ] https://example.com trả về 200
- [ ] Response time < 3s
- [ ] SSL certificate còn > 30 ngày

## API
- [ ] /health endpoint trả về {"status": "ok"}
- [ ] Database connection pool < 80%

## Business
- [ ] Có đơn hàng mới trong 24h qua
- [ ] Không có ticket support P1 chưa xử lý
```

### Cách Hoạt Động

1. Cron trigger heartbeat mỗi 30 phút
2. Agent đọc HEARTBEAT.md → chạy từng check
3. Dùng `web_fetch`, `exec`, `browser` để verify
4. Nếu tất cả OK → im lặng
5. Nếu có vấn đề → `message` user qua Telegram/Slack ngay lập tức

### Setup Heartbeat

```json
{
  "name": "heartbeat-prod",
  "schedule": { "kind": "every", "everyMs": 1800000 },
  "payload": { "kind": "agent", "wakeHeartbeat": true }
}
```

`wakeHeartbeat: true` → Agent dùng model nhẹ (giảm cost) và chỉ chạy HEARTBEAT.md checklist.

## Proactive Messaging — Agent Chủ Động Nhắn

Cron + `message` tool = Agent chủ động nhắn tin cho user:

```
Agent chạy cron → phát hiện issue → message tool:
{
  "channel": "telegram",
  "to": "user-telegram-id",
  "text": "⚠️ Website down! https://example.com trả về 503 từ 14:32."
}
```

Không cần user hỏi — agent tự phát hiện và báo.

## Quản Lý Cron

| Method | Mô tả |
|---|---|
| `cron.list` | Xem tất cả jobs |
| `cron.toggle` | Bật/tắt job |
| `cron.run` | Chạy ngay (không đợi lịch) |
| `cron.runs` | Xem lịch sử chạy |
| `cron.delete` | Xóa job |

## Tiếp Theo

- [Công thức hay](07-cong-thuc-hay.md) — 10 cách dùng GoClaw sáng tạo
