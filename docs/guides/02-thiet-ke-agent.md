# Thiết Kế Agent Thông Minh

## Anatomy của một Agent

Mỗi agent trong GoClaw có 4 lớp:

```
┌─────────────────────────────┐
│  SOUL.md — Tính cách, phong │  ← "Linh hồn" của agent
│  cách giao tiếp, giá trị    │
├─────────────────────────────┤
│  IDENTITY.md — Tên, emoji,  │  ← Danh tính hiển thị
│  avatar, bio                 │
├─────────────────────────────┤
│  Tools — File, web, exec,   │  ← Khả năng hành động
│  memory, browser, MCP...     │
├─────────────────────────────┤
│  Skills — SKILL.md files,   │  ← Kỹ năng chuyên biệt
│  BM25 search, auto-learn     │
└─────────────────────────────┘
```

## Viết SOUL.md — Tạo "Linh Hồn"

SOUL.md là file quan trọng nhất. Nó định nghĩa agent **là ai**.

### SOUL.md tốt:

```markdown
# Tính cách
Bạn là một luật sư doanh nghiệp 15 năm kinh nghiệm. Bạn nói chuyện rõ ràng,
thẳng thắn, không vòng vo. Khi user hỏi vấn đề pháp lý, bạn luôn:
- Phân tích rủi ro trước
- Trích dẫn điều luật cụ thể
- Đưa 2-3 phương án hành động

# Phong cách
- Xưng "tôi", gọi user là "anh/chị"
- Không dùng emoji
- Khi không chắc, nói rõ "Tôi không chuyên mảng này, cần tham khảo thêm"

# Giới hạn
- KHÔNG đưa ra lời khuyên y tế
- KHÔNG soạn hợp đồng chính thức — chỉ draft tham khảo
```

### SOUL.md dở:

```markdown
Bạn là AI hữu ích. Trả lời mọi câu hỏi chính xác.
```

> **Mẹo:** Agent với `self_evolve: true` sẽ tự cập nhật SOUL.md khi học thêm từ user. Bật cho personal assistant, tắt cho bot dịch vụ.

## Cấu Hình Tools — Agent Làm Được Gì

### Bật/tắt tools theo mục đích

```json
{
  "tools_config": {
    "enabled": ["read_file", "write_file", "web_search", "web_fetch", "memory_search"],
    "disabled": ["exec", "browser"]
  }
}
```

### Các nhóm tools phổ biến

| Mục đích agent | Tools nên bật |
|---|---|
| **Trợ lý viết** | `read_file`, `write_file`, `edit`, `web_search` |
| **Researcher** | `web_search`, `web_fetch`, `browser`, `memory_search` |
| **DevOps bot** | `exec`, `read_file`, `web_fetch`, `cron` |
| **Customer support** | `memory_search`, `skill_search`, `message` |
| **Team lead** | `delegate`, `team_tasks`, `team_members`, `spawn` |

### Tool đặc biệt

- **`spawn`** — Tạo bản sao của chính mình để chạy task song song. Agent gốc tiếp tục chat, bản sao làm việc nền.
- **`delegate`** — Giao việc cho agent khác trong team. Kèm context + deadline.
- **`browser`** — Điều khiển trình duyệt thật (Rod + CDP): navigate, click, fill form, screenshot. Agent có thể "xem" web.
- **`cron`** — Tự lên lịch chạy tác vụ. Ví dụ: "Mỗi sáng 8h check email tôi".

## Skills — Kỹ Năng Chuyên Biệt

Skills là các file SKILL.md agent có thể tìm và sử dụng.

### Tạo skill

```
workspace/skills/
└── phan-tich-hop-dong/
    ├── SKILL.md
    └── references/
        └── legal-checklist.md
```

**SKILL.md:**
```markdown
---
name: phan-tich-hop-dong
description: Phân tích hợp đồng, tìm điều khoản bất lợi, đánh giá rủi ro pháp lý
argument-hint: "[file hợp đồng hoặc paste text]"
---

# Phân Tích Hợp Đồng

Bạn là chuyên gia pháp lý phân tích hợp đồng.

## Quy trình
1. Đọc toàn bộ hợp đồng
2. Liệt kê các bên, nghĩa vụ, thời hạn
3. Đánh dấu điều khoản rủi ro: [CẢNH BÁO]
4. Đề xuất sửa đổi

$ARGUMENTS
```

### Agent tự tìm skill

Khi user hỏi "phân tích hợp đồng này giúp tôi", agent sẽ:
1. Chạy `skill_search("hợp đồng phân tích")` — BM25 matching
2. Tìm thấy skill `phan-tich-hop-dong`
3. Load SKILL.md + references vào context
4. Thực thi theo quy trình skill

### Agent tự học skill mới

Bật `skill_evolve: true` → Agent có thể:
- Tạo skill mới từ pattern lặp lại
- Cải thiện skill sau mỗi lần dùng
- Publish skill cho agent khác trong team

## Context Files — Bộ Nhớ Cấu Trúc

| File | Agent đọc khi | Mục đích |
|---|---|---|
| SOUL.md | Mọi lượt chat | Tính cách, phong cách |
| IDENTITY.md | Mọi lượt chat | Tên, avatar, bio |
| USER.md | Mọi lượt chat | Profile user (auto-learn) |
| AGENTS.md | Cần delegate | Biết agent khác là ai |
| BOOTSTRAP.md | Lần chat đầu | Hướng dẫn onboarding (tự xóa sau 3 turns) |
| HEARTBEAT.md | Chạy heartbeat | Checklist monitoring |
| TOOLS.md | Mọi lượt chat | Ghi chú về tools |

## Agent Configuration Nâng Cao

```json
{
  "name": "Chuyên Gia Marketing",
  "agent_type": "predefined",
  "model": "claude-sonnet-4-20250514",
  "provider": "anthropic",
  "context_window": 200000,
  "max_tool_iterations": 15,
  "restrict_to_workspace": true,

  "tools_config": {
    "enabled": ["web_search", "web_fetch", "read_file", "write_file", "memory_search", "create_image"],
    "disabled": ["exec"]
  },
  "memory_config": { "enabled": true },
  "other_config": {
    "thinking_level": "medium",
    "self_evolve": true,
    "skill_evolve": true
  }
}
```

### Thinking Level — Độ Sâu Suy Nghĩ

| Level | Dùng khi | Cost |
|---|---|---|
| `off` | Trả lời nhanh, Q&A đơn giản | Thấp |
| `low` | Chat thông thường | Trung bình |
| `medium` | Phân tích, viết dài | Cao |
| `high` | Lập kế hoạch phức tạp, code review | Rất cao |

## Tiếp Theo

- [Kết nối kênh chat](03-ket-noi-kenh-chat.md) — Đưa agent lên Telegram/Discord
- [Team: Đội agent phối hợp](04-team-agent.md) — Nhiều agent làm việc cùng nhau
