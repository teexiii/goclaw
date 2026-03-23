# Bộ Nhớ & Knowledge Graph

## Vấn Đề

LLM "quên" mọi thứ sau mỗi lượt chat. Session history giúp nhớ trong cuộc trò chuyện, nhưng qua session mới = mất hết.

GoClaw giải quyết bằng 3 lớp nhớ:

```
┌───────────────────────────┐
│  Session History          │  ← Ngắn hạn: cuộc chat hiện tại
│  (auto-summarize > 75%)   │
├───────────────────────────┤
│  Memory (pgvector)        │  ← Dài hạn: facts, preferences, context
│  Semantic search           │
├───────────────────────────┤
│  Knowledge Graph          │  ← Cấu trúc: entities + relationships
│  Entity → Relation → Entity│
└───────────────────────────┘
```

## Memory — Bộ Nhớ Ngữ Nghĩa

### Cách hoạt động

1. Agent quyết định lưu thông tin quan trọng → `memory_store`
2. Text được chunk → embedding (vector hóa) → lưu pgvector
3. Lần sau cần nhớ → `memory_search("keyword")` → cosine similarity → top results

### Bật memory cho agent

```json
{
  "memory_config": { "enabled": true }
}
```

### Agent tự nhớ

Khi chat với user, agent tự phát hiện thông tin đáng nhớ:

```
User: "Tôi thích cà phê đen, không đường"
Agent: [lưu memory: user preference - cà phê đen không đường]

--- 3 tuần sau, session mới ---

User: "Gợi ý quán cà phê ở Q1"
Agent: [memory_search("cà phê") → nhớ: bạn thích cà phê đen không đường]
       "Đây là 3 quán có espresso ngon ở Q1, phù hợp với sở thích đen không đường của bạn..."
```

### Phạm vi memory

| Agent type | Memory scope |
|---|---|
| Open | Per-user — mỗi user có memory riêng |
| Predefined | Tùy config — shared hoặc per-user |
| Team | Team-scoped — agent nhớ trong context team |

## Knowledge Graph — Đồ Thị Tri Thức

Khác memory (text tự do), Knowledge Graph lưu **quan hệ có cấu trúc**:

```
[Minh] --works_at--> [TechCorp]
[Minh] --manages--> [Team Alpha]
[TechCorp] --competitor_of--> [InnoTech]
[Team Alpha] --building--> [Project Phoenix]
```

### Agent tạo và truy vấn KG

```
User: "Minh ở TechCorp vừa chuyển sang InnoTech"

Agent:
  1. knowledge_graph_search("Minh TechCorp")
  2. Xóa relation: Minh --works_at--> TechCorp
  3. Thêm relation: Minh --works_at--> InnoTech
  4. Thêm observation: "Minh chuyển việc tháng 3/2026"
```

### Dùng KG cho gì?

- **CRM bot**: Theo dõi khách hàng, contacts, deals
- **Research assistant**: Map quan hệ giữa companies, people, events
- **Project manager**: Dependencies giữa tasks, teams, deliverables
- **Novel writing**: Nhân vật, quan hệ, sự kiện (dùng với bộ skills viết truyện!)

## Auto-Summarization — Tự Tóm Tắt

Khi session history vượt 75% context window:

1. Agent tự động summarize lịch sử cũ
2. Summary thay thế messages chi tiết
3. Giữ messages gần nhất nguyên vẹn
4. User không bị giới hạn độ dài cuộc chat

### Compaction config

```json
{
  "compaction_config": {
    "enabled": true,
    "threshold": 0.75
  }
}
```

## Context Pruning — Cắt Bớt Thông Minh

Khác summarization (tóm lại), pruning **cắt bỏ** tool results cũ để tiết kiệm context:

```json
{
  "context_pruning": {
    "enabled": true,
    "keep_recent": 5
  }
}
```

Agent nhớ mình đã gọi tool gì, nhưng kết quả cũ bị cắt → tiết kiệm token đáng kể cho agent dùng nhiều tools.

## USER.md — Profile Tự Học

Ngoài memory, agent có USER.md per-user — file text tự cập nhật:

```markdown
# User Profile

## Thông tin cơ bản
- Tên: Tùng
- Vai trò: Product Manager tại startup edtech
- Timezone: Asia/Ho_Chi_Minh

## Sở thích giao tiếp
- Thích bullet points, không thích văn dài
- Hay hỏi về metric và ROI
- Xưng hô: "anh" - "tôi"

## Chủ đề thường hỏi
- Product strategy
- User research methods
- Competitor analysis
```

Agent tự cập nhật USER.md qua `write_file` khi học thêm về user.

## Tips

1. **Bật memory cho personal assistant** — Agent nhớ sở thích, lịch sử, context dài hạn
2. **Dùng KG cho CRM/research** — Khi cần truy vấn quan hệ phức tạp
3. **Team memory riêng team** — Agent trong team nhớ context chung, không lẫn với chat cá nhân
4. **Đừng nhớ mọi thứ** — Agent cần biết phân biệt thông tin quan trọng vs tạm thời

## Tiếp Theo

- [Lên lịch tự động](06-cron-va-heartbeat.md)
- [Công thức hay](07-cong-thuc-hay.md)
