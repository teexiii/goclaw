# Team: Đội Agent Phối Hợp

## Ý Tưởng

Thay vì 1 agent "biết tất cả", bạn tạo team chuyên biệt — mỗi agent giỏi 1 việc, phối hợp qua task board.

```
┌─────────────────────────────────────────────┐
│                 TEAM BOARD                   │
│                                              │
│  📋 Task: "Phân tích đối thủ cạnh tranh"    │
│  ├── Assigned: Researcher                    │
│  ├── Status: in_progress                     │
│  └── Blocked by: (none)                      │
│                                              │
│  📋 Task: "Viết báo cáo marketing Q1"       │
│  ├── Assigned: Writer                        │
│  ├── Status: blocked                         │
│  └── Blocked by: Task #1                     │
│                                              │
│  📋 Task: "Review & format báo cáo"         │
│  ├── Assigned: Editor                        │
│  ├── Status: pending                         │
│  └── Blocked by: Task #2                     │
└─────────────────────────────────────────────┘
```

## Tạo Team

### Bước 1: Tạo các agent chuyên biệt

```json
// Agent 1: Team Lead — điều phối, phân task
{
  "name": "PM Bot",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": ["delegate", "team_tasks", "team_members", "spawn", "web_search"]
  }
}

// Agent 2: Researcher — tìm kiếm thông tin
{
  "name": "Researcher",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": ["web_search", "web_fetch", "browser", "write_file", "memory_search"]
  }
}

// Agent 3: Writer — viết nội dung
{
  "name": "Writer",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": ["read_file", "write_file", "edit", "memory_search"]
  }
}
```

### Bước 2: Tạo team

```json
{
  "method": "teams.create",
  "params": {
    "name": "Marketing Team",
    "lead_agent_id": "uuid-of-pm-bot",
    "members": [
      { "agent_id": "uuid-of-researcher", "role": "member" },
      { "agent_id": "uuid-of-writer", "role": "member" }
    ],
    "description": "Đội marketing: research, viết content, review"
  }
}
```

### Bước 3: Giao việc

Chat với PM Bot:

> "Phân tích 3 đối thủ chính trong ngành edtech Việt Nam, rồi viết báo cáo tổng hợp"

PM Bot sẽ:
1. Tạo task "Research đối thủ" → assign cho Researcher
2. Tạo task "Viết báo cáo" → assign cho Writer, blocked by task #1
3. Theo dõi progress qua `team_tasks`
4. Khi Researcher xong → Writer tự unblock → bắt đầu viết
5. Tổng hợp kết quả trả cho bạn

## Vai Trò Trong Team

| Role | Quyền |
|---|---|
| **Lead** | Tạo/assign task, delegate, approve/reject deliverables |
| **Member** | Nhận task, complete task, gửi deliverables |
| **Reviewer** | Review deliverables, approve/reject |

## Task Lifecycle

```
pending → in_progress → in_review → completed
                ↓                       ↑
              blocked ──────────────────┘
                ↓
              failed → (retry) → in_progress
```

- **pending**: Chờ agent nhận
- **in_progress**: Agent đang làm
- **blocked**: Chờ task khác xong
- **in_review**: Đợi lead/reviewer duyệt
- **completed**: Xong
- **failed**: Lỗi (có thể retry)
- **stale**: Quá lâu không update

## Delegation — Giao Việc Trực Tiếp

Ngoài task board, agent có thể delegate trực tiếp:

```
User → PM Bot: "Tìm giá vé máy bay HN-SGN tuần sau"
PM Bot → delegate("Researcher", "Tìm giá vé HN-SGN ngày X-Y")
Researcher → web_search + web_fetch → trả kết quả
PM Bot → tổng hợp → trả user
```

**2 chế độ delegation:**
- **Sync**: PM Bot đợi Researcher trả lời (blocking)
- **Async**: PM Bot tiếp tục làm việc khác, nhận kết quả sau

## Shared Workspace — Không Gian Chung

Team có workspace chung để chia sẻ file:

```
teams/{teamId}/
├── research/
│   ├── doi-thu-a.md
│   └── doi-thu-b.md
├── drafts/
│   └── bao-cao-q1.md
└── final/
    └── bao-cao-q1-final.md
```

Agent dùng `read_file` / `write_file` trên team workspace. Mọi thành viên đều thấy.

## AGENTS.md — Biết Nhau Để Phối Hợp

Mỗi agent có file AGENTS.md mô tả các agent khác:

```markdown
# Agents Trong Team

## Researcher
- Chuyên tìm kiếm thông tin, phân tích dữ liệu
- Giỏi: web research, data extraction, competitor analysis
- Giao việc qua: delegate("Researcher", "mô tả task")

## Writer
- Chuyên viết content marketing, báo cáo, blog post
- Giỏi: copywriting, report writing, content strategy
- Giao việc qua: delegate("Writer", "mô tả task")
```

## Team Messages — Nhắn Tin Nội Bộ

Agent gửi tin nhắn cho nhau qua mailbox:

- Lead gửi feedback: "Phần research cần thêm số liệu revenue"
- Member hỏi clarification: "Báo cáo cần format PDF hay Markdown?"
- Reviewer gửi revision: "Sửa lại phần kết luận"

## Ví Dụ Team Thực Tế

### Team Content Marketing

```
Lead: "Content Manager" — phân bổ topic, review bài viết
├── "SEO Researcher" — keyword research, competitor content analysis
├── "Copywriter" — viết blog post, social media content
└── "Editor" — chỉnh sửa, đảm bảo tone of voice, SEO optimization
```

### Team Customer Support

```
Lead: "Support Manager" — routing, escalation, SLA tracking
├── "L1 Support" — trả lời FAQ, hướng dẫn cơ bản
├── "Technical Support" — xử lý bug report, troubleshooting
└── "Billing Support" — hoàn tiền, nâng cấp, invoice
```

### Team Software Dev

```
Lead: "Tech Lead" — thiết kế, code review, task breakdown
├── "Backend Dev" — API, database, business logic
├── "Frontend Dev" — UI, UX, responsive design
└── "QA" — test plan, bug hunting, regression
```

## Tiếp Theo

- [Bộ nhớ & Knowledge Graph](05-bo-nho-agent.md)
- [Lên lịch tự động](06-cron-va-heartbeat.md)
