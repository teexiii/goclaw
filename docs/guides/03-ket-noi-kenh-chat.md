# Kết Nối Kênh Chat

## Tổng Quan Kênh

GoClaw hỗ trợ 7 kênh, mỗi kênh có tính năng khác nhau:

| Kênh | Streaming | Reactions | Voice/STT | Media | Pairing |
|---|---|---|---|---|---|
| **WebSocket** (Web UI) | ✅ | — | — | ✅ | — |
| **Telegram** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Discord** | ✅ | ✅ | ✅ | ✅ | — |
| **Slack** | ✅ | ✅ | — | ✅ | — |
| **Feishu/Lark** | — | — | — | ✅ | — |
| **WhatsApp** | — | — | — | ✅ | — |
| **Zalo** (OA + Personal) | — | — | — | ✅ | — |

## Telegram — Kênh Mạnh Nhất

Telegram là kênh được hỗ trợ đầy đủ nhất. Nếu chỉ chọn một kênh, chọn Telegram.

### Setup

1. Chat @BotFather trên Telegram → `/newbot` → lấy token
2. Thêm channel instance qua Web UI hoặc RPC:

```json
{
  "method": "channels.create",
  "params": {
    "name": "telegram-main",
    "channel_type": "telegram",
    "agent_id": "uuid-of-agent",
    "credentials": {
      "bot_token": "123456:ABC-DEF..."
    },
    "config": {
      "dm_policy": "pairing",
      "group_policy": "open"
    }
  }
}
```

### DM Policy — Ai Được Nói Chuyện?

| Policy | Hành vi |
|---|---|
| `open` | Ai cũng chat được |
| `pairing` | Phải nhập mã ghép nối trước |
| `allowlist` | Chỉ danh sách đã duyệt |
| `disabled` | Tắt DM |

**Pairing flow:**
1. User gửi `/start` cho bot → Bot trả lời "Nhập mã ghép nối"
2. Admin vào Web UI → Devices → Generate code (6 ký tự, hết hạn 10 phút)
3. User nhập mã → Bot xác nhận → Từ giờ user chat thoải mái

### Tính Năng Đặc Biệt

- **Streaming**: Agent nghĩ đến đâu, tin nhắn cập nhật đến đó (edit message real-time)
- **Reactions**: 🤔 đang nghĩ → ⚡ đang dùng tool → ✅ xong (hoặc ❌ lỗi)
- **Voice**: Gửi voice message → STT tự động → Agent nhận text
- **Topics**: Trong group có Topics, mỗi topic = 1 session riêng
- **Formatting**: Markdown → HTML Telegram (bảng tự chuyển thành `<pre>` ASCII)

## Discord

### Setup

1. [Discord Developer Portal](https://discord.com/developers) → New Application → Bot → Copy Token
2. Bật Intents: Message Content, Guild Messages, DM Messages
3. Invite bot vào server với permissions: Send Messages, Read Message History, Add Reactions, Attach Files

```json
{
  "channel_type": "discord",
  "credentials": { "bot_token": "..." },
  "config": { "dm_policy": "open" }
}
```

### Tính năng

- Streaming qua message edit
- Reactions trạng thái
- Thread support — mỗi thread = 1 session
- File upload/download

## Slack

### Setup

1. [api.slack.com/apps](https://api.slack.com/apps) → Create App
2. OAuth Scopes: `chat:write`, `channels:history`, `groups:history`, `im:history`, `files:read`, `files:write`, `reactions:write`
3. Event Subscriptions: `message.channels`, `message.groups`, `message.im`

```json
{
  "channel_type": "slack",
  "credentials": {
    "bot_token": "xoxb-...",
    "signing_secret": "..."
  }
}
```

## Feishu/Lark

Dùng cho tổ chức sử dụng hệ sinh thái Lark (ByteDance). Setup qua Lark Developer Console.

```json
{
  "channel_type": "feishu",
  "credentials": {
    "app_id": "...",
    "app_secret": "..."
  }
}
```

## Zalo

### Zalo OA (Official Account)

Cho doanh nghiệp có tài khoản OA:

```json
{
  "channel_type": "zalo",
  "credentials": {
    "oa_id": "...",
    "access_token": "..."
  }
}
```

### Zalo Personal

Dùng tài khoản cá nhân (QR code auth):

```json
{
  "channel_type": "zalo_personal",
  "credentials": {
    "imei": "...",
    "cookies": "..."
  }
}
```

## Một Agent, Nhiều Kênh

Một agent có thể nhận tin từ nhiều kênh cùng lúc:

```
Agent "Trợ Lý"
├── telegram-main     (DM + groups)
├── discord-server     (server channels)
└── slack-workspace    (workspace channels)
```

Session key phân biệt kênh:
- `agent:{id}:telegram:direct:{userId}` — Telegram DM
- `agent:{id}:discord:group:{channelId}` — Discord channel
- `agent:{id}:slack:group:{channelId}` — Slack channel

User A chat qua Telegram và Discord = 2 session riêng biệt.

## Session Scope — Quản Lý Phạm Vi Hội Thoại

| Scope | Hành vi | Dùng khi |
|---|---|---|
| `per-sender` | Mỗi user 1 session (mặc định) | Đa số trường hợp |
| `per-channel-peer` | Mỗi user mỗi kênh 1 session | Cần tách context theo kênh |
| `global` | Tất cả chung 1 session | Agent trả lời giống nhau cho mọi người |

## Tiếp Theo

- [Team: Đội agent phối hợp](04-team-agent.md)
- [Công thức hay](07-cong-thuc-hay.md) — Ví dụ setup Telegram bot thực tế
