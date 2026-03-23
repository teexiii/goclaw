# 10 Công Thức Hay với GoClaw

## 1. Trợ Lý Cá Nhân Biết Tuốt

**Setup:** 1 open agent + memory + Telegram

```
Agent: "Jarvis"
Type: open
Tools: web_search, web_fetch, memory_search, cron, message, read_file, write_file
Memory: enabled
Self-evolve: true
Channel: Telegram (DM policy: pairing)
```

**Hay ở chỗ:** Agent tự học profile bạn qua USER.md, nhớ sở thích qua memory, tự lên lịch nhắc nhở. Càng dùng càng hiểu bạn.

**Chat thử:**
- "Nhắc tôi họp lúc 3h chiều mai" → agent tạo cron at job
- "Tìm nhà hàng Nhật ngon ở Q2" → web search + nhớ bạn thích sushi lần trước
- "Tóm tắt email tuần này" → nếu có MCP Gmail server

---

## 2. Bot Hỗ Trợ Khách Hàng 24/7

**Setup:** 1 predefined agent + skills + multi-channel

```
Agent: "Support Bot"
Type: predefined (tính cách cố định cho mọi user)
SOUL.md: Chuyên nghiệp, kiên nhẫn, xưng "em" gọi "anh/chị"
Skills: faq-lookup, ticket-create, order-status
Tools: memory_search, skill_search, message, web_fetch
Channels: Telegram (open) + Zalo OA + Website (WebSocket)
```

**Hay ở chỗ:** Cùng 1 agent phục vụ trên 3 kênh. SOUL.md giữ tính cách nhất quán. Skills chứa quy trình xử lý. Memory nhớ lịch sử từng khách.

---

## 3. Team Viết Content Marketing

**Setup:** 3 agent team

```
Team: "Content Factory"
├── Lead: "Content Manager" — phân bổ, review, approve
├── Member: "Researcher" — keyword research, competitor analysis
└── Member: "Copywriter" — viết bài, social post

Workflow:
  User → "Viết 3 bài blog về AI trong giáo dục"
  Manager → tạo 3 tasks, assign Researcher trước
  Researcher → research keywords, audiences → write file
  Manager → assign Copywriter, kèm research data
  Copywriter → viết 3 bài → submit review
  Manager → review → approve/reject → trả user
```

**Hay ở chỗ:** Pipeline tự động. Bạn chỉ cần 1 câu, team 3 agent xử lý đến bước cuối.

---

## 4. Monitoring Dashboard Thông Minh

**Setup:** 1 agent + heartbeat + cron + proactive message

```
Agent: "SRE Bot"
Tools: web_fetch, exec, browser, cron, message
HEARTBEAT.md: checklist 10 endpoints + DB + queue size
Cron: every 15 minutes (heartbeat)
Alert channel: Telegram group "oncall-team"
```

**Hay ở chỗ:** Không phải Grafana/PagerDuty — agent **hiểu** context. Khi website slow, nó check DB, check queue, check recent deploys, rồi gửi alert kèm **phân tích nguyên nhân**.

---

## 5. Nghiên Cứu Thị Trường Tự Động

**Setup:** 1 agent + browser + memory + cron

```
Agent: "Market Intel"
Tools: web_search, web_fetch, browser, memory_search, write_file
Memory: enabled
Cron: weekly (thứ 2, 9h sáng)
Message: "Scan 10 đối thủ: giá, tính năng mới, đánh giá user.
          So sánh với tuần trước (dùng memory).
          Viết báo cáo vào workspace/reports/weekly-{date}.md"
```

**Hay ở chỗ:** Agent dùng browser thật để navigate website đối thủ, screenshot pricing page, so sánh với memory tuần trước → phát hiện thay đổi.

---

## 6. Agent Viết Tiểu Thuyết

**Setup:** 1 open agent + 11 skills viết truyện

```
Agent: "Nhà Văn AI"
Type: open
Skills: viet-truyen, tao-nhan-vat, cot-truyen, the-gioi,
        hoi-thoai, mo-ta-canh, bien-tap, dat-ten,
        dong-thoi-gian, logic-truyen, nghien-cuu
Tools: read_file, write_file, edit, web_search, memory_search, skill_search
Memory: enabled (nhớ nhân vật, plot points, worldbuilding)
```

**Hay ở chỗ:** Bộ 11 skills chuyên biệt cho từng khía cạnh viết truyện. Agent dùng `skill_search` tự tìm skill phù hợp. Memory nhớ toàn bộ thế giới truyện across sessions.

**Chat thử:**
- "Tạo nhân vật phản diện cho truyện kiếm hiệp" → skill `tao-nhan-vat`
- "Viết chương 5: Minh phát hiện bí mật" → skill `viet-truyen` + đọc chương 1-4
- "Check logic từ chương 1-5" → skill `logic-truyen` + mermaid diagrams

---

## 7. DevOps Co-pilot

**Setup:** 1 agent + exec + MCP servers

```
Agent: "DevOps Bot"
Tools: exec, read_file, web_fetch, cron, message
MCP servers: github-mcp, docker-mcp, k8s-mcp
Restrict to workspace: false (cần truy cập system)
Shell deny groups: giữ mặc định (block rm -rf, halt, etc.)
```

**Hay ở chỗ:** Agent chạy lệnh thật trên server. MCP bridge kết nối GitHub API, Docker, K8s. Kết hợp cron để auto-deploy khi merge vào main.

**Chat thử:**
- "Deploy branch feature/auth lên staging"
- "Check logs 1h qua, tìm error 500"
- "Mỗi khi có PR mới vào main, auto run test rồi deploy staging"

---

## 8. CRM Bot Cho Sales Team

**Setup:** 1 predefined agent + knowledge graph + multi-user

```
Agent: "Sales Assistant"
Type: predefined
Tools: knowledge_graph_search, memory_search, web_search, message, cron
KG: entities = contacts, companies, deals
Memory: shared (cả team đều thấy)
Channel: Slack workspace
```

**Hay ở chỗ:** Knowledge Graph lưu quan hệ cấu trúc (contact → company → deal). Mọi người trong sales team chat cùng agent, agent nhớ context chung.

**Chat thử:**
- "Thêm contact: Anh Minh, CTO TechCorp, gặp hôm qua ở sự kiện AI"
- "Ai ở TechCorp mình đã liên hệ?" → KG query
- "Nhắc tôi follow up anh Minh sau 3 ngày" → cron

---

## 9. Dịch Thuật + Localization Pipeline

**Setup:** Team 2 agent

```
Lead: "Translation Manager"
Member: "Translator" (predefined, SOUL.md = chuyên gia dịch thuật)

Workflow:
  User upload file .md tiếng Anh
  Manager → tách thành chunks → delegate cho Translator
  Translator → dịch từng chunk → write_file
  Manager → merge + review consistency → trả user
```

**Hay ở chỗ:** Agent đọc toàn bộ file trước khi dịch (hiểu context). Dùng memory nhớ thuật ngữ đã dịch trước đó để nhất quán. Team có thể mở rộng: thêm "Reviewer" agent check chất lượng.

---

## 10. Agent Tự Evolve — Ngày Càng Giỏi

**Setup:** 1 agent + self_evolve + skill_evolve

```json
{
  "other_config": {
    "self_evolve": true,
    "skill_evolve": true
  }
}
```

**self_evolve:** Agent tự cập nhật SOUL.md khi phát hiện cách tiếp cận tốt hơn.

**skill_evolve:** Agent tự tạo SKILL.md mới khi phát hiện pattern lặp lại.

**Hay ở chỗ:** Bạn dùng agent 1 tháng → nó tự tạo 5-10 skills từ những việc bạn hay nhờ. Tháng sau nó xử lý nhanh hơn vì đã có skill sẵn.

---

## Kết Hợp Tất Cả

GoClaw mạnh nhất khi kết hợp:

```
Agent + Skills        = Chuyên gia lĩnh vực cụ thể
Agent + Memory        = Nhớ mọi thứ, càng dùng càng giỏi
Agent + Cron          = Tự động, không cần nhắc
Agent + Channels      = Ở đâu cũng tiếp cận được
Agent + Team          = Chia để trị, pipeline tự động
Agent + MCP           = Kết nối mọi hệ thống bên ngoài
Agent + Browser       = "Thấy" và tương tác web thật
Agent + KG            = Hiểu quan hệ phức tạp
Agent + Self-evolve   = Tự cải thiện theo thời gian
```

> **Nguyên tắc vàng:** Bắt đầu đơn giản (1 agent, 1 channel, vài tools). Thêm tính năng khi thực sự cần. Đừng over-engineer ngay từ đầu.
