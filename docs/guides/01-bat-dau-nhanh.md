# Bắt Đầu Nhanh với GoClaw

## GoClaw là gì?

GoClaw là gateway AI đa kênh — bạn tạo agent thông minh, kết nối vào Telegram/Discord/Slack/Zalo, rồi để chúng làm việc cho bạn. Mỗi agent có bộ nhớ riêng, kỹ năng riêng, và có thể phối hợp theo team.

## Cài đặt 5 phút

```bash
# Build + chạy wizard
go build -o goclaw .
./goclaw onboard        # Wizard hỏi: DB, key mã hóa, provider...
source .env.local       # Load secrets
./goclaw migrate up     # Tạo tables
./goclaw                # Start gateway
```

Wizard sẽ tạo sẵn:
- File `config.json5` với cấu hình mặc định
- File `.env.local` chứa secrets (DB DSN, encryption key)
- Provider placeholder để bạn cấu hình sau qua Web UI

## Tạo Agent Đầu Tiên

### Qua Web UI
```bash
cd ui/web && pnpm install && pnpm dev
# Mở http://localhost:5173 → Agents → New Agent
```

### Qua WebSocket RPC
```json
{
  "method": "agents.create",
  "params": {
    "name": "Trợ Lý Cá Nhân",
    "agent_type": "open",
    "emoji": "🧠"
  }
}
```

### Qua HTTP API
```bash
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name": "Trợ Lý Cá Nhân", "agent_type": "open"}'
```

## 2 Loại Agent

| | Open Agent | Predefined Agent |
|---|---|---|
| **Dùng khi** | Mỗi user tùy biến agent theo ý mình | Agent có tính cách cố định cho mọi user |
| **Context files** | 7 file riêng per user | SOUL/IDENTITY chung, chỉ USER.md riêng |
| **Ví dụ** | Trợ lý cá nhân, notepad AI | Bot hỗ trợ khách hàng, chuyên gia pháp lý |

## Kết Nối Kênh Chat

Agent tạo xong chưa nói chuyện được — cần gắn vào kênh:

1. **WebSocket** — Dùng ngay qua Web UI (mặc định)
2. **Telegram** — Tạo bot qua @BotFather, thêm token vào channel instance
3. **Discord** — Tạo app, thêm bot token
4. **Slack** — Tạo app + signing secret
5. **Zalo OA** — Đăng ký OA, lấy access token

Chi tiết: [Kết nối kênh chat](03-ket-noi-kenh-chat.md)

## Chat Thử

Sau khi start gateway + mở Web UI:

1. Chọn agent từ sidebar
2. Gõ tin nhắn
3. Agent sẽ:
   - Đọc SOUL.md (tính cách) + IDENTITY.md (danh tính)
   - Chạy think → act → observe loop
   - Stream response real-time
   - Lưu lịch sử vào session

## Tiếp Theo

- [Thiết kế Agent thông minh](02-thiet-ke-agent.md) — SOUL.md, tools, skills
- [Kết nối kênh chat](03-ket-noi-kenh-chat.md) — Telegram, Discord, Zalo...
- [Team: Đội agent phối hợp](04-team-agent.md) — Delegation, tasks, shared workspace
- [Bộ nhớ & Knowledge Graph](05-bo-nho-agent.md) — Agent nhớ mọi thứ
- [Lên lịch tự động](06-cron-va-heartbeat.md) — Cron jobs + heartbeat monitoring
- [Công thức hay](07-cong-thuc-hay.md) — 10 cách dùng GoClaw sáng tạo
