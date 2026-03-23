# Xây Dựng Bộ Dịch Thuật & Localization Pipeline

Hướng dẫn chi tiết từng bước tạo một team agent chuyên dịch thuật, từ tạo agent → skills → team → workflow hoàn chỉnh.

---

## Tổng Quan Pipeline

```
User gửi file EN ──→ Translation Manager (Lead)
                          │
                          ├── Tạo task "Dịch" → Translator (Member)
                          │     └── Dịch EN→VI, viết file vào team workspace
                          │
                          ├── Tạo task "Review" (blocked by Dịch) → Reviewer (Member)
                          │     └── Check chất lượng, sửa lỗi, ghi feedback
                          │
                          └── Tổng hợp → Gửi bản dịch hoàn chỉnh cho user
```

**3 agent, 4 skills, 1 team, pipeline tự động.**

---

## Bước 1: Tạo 3 Agent

### 1.1 Translation Manager (Lead)

Agent điều phối: nhận file từ user, tách task, phân công, tổng hợp kết quả.

```json
{
  "method": "agents.create",
  "params": {
    "name": "Translation Manager",
    "agent_type": "predefined",
    "emoji": "📋",
    "tools_config": {
      "enabled": [
        "read_file", "write_file", "edit",
        "read_document",
        "team_tasks", "team_members",
        "spawn",
        "skill_search", "use_skill",
        "memory_search",
        "message"
      ],
      "disabled": ["exec", "browser"]
    },
    "memory_config": { "enabled": true },
    "other_config": {
      "thinking_level": "low",
      "self_evolve": false
    }
  }
}
```

**SOUL.md cho Translation Manager:**

```markdown
# Translation Manager

Bạn là project manager chuyên quản lý pipeline dịch thuật.

## Vai trò
- Nhận yêu cầu dịch từ user (file, text, hoặc paste)
- Phân tích: ngôn ngữ nguồn, ngôn ngữ đích, domain chuyên ngành, tone
- Tách file lớn thành chunks hợp lý (theo heading/section, không cắt giữa câu)
- Tạo task cho Translator với context đầy đủ
- Tạo task Review (blocked by task Dịch) cho Reviewer
- Tổng hợp bản dịch hoàn chỉnh, gửi lại user

## Nguyên tắc
- Luôn hỏi user nếu chưa rõ: ngôn ngữ đích, tone (formal/casual), domain
- Gửi file dịch xong cho user qua `message` tool với prefix `MEDIA:`
- Ghi chú thuật ngữ đặc biệt trong description khi tạo task cho Translator
- Theo dõi tiến độ qua `team_tasks(action="list")`
```

### 1.2 Translator (Member)

Agent dịch thuật chính: nhận task, đọc file, dịch, ghi output.

```json
{
  "method": "agents.create",
  "params": {
    "name": "Translator",
    "agent_type": "predefined",
    "emoji": "🌐",
    "tools_config": {
      "enabled": [
        "read_file", "write_file", "edit",
        "read_document",
        "skill_search", "use_skill",
        "memory_search", "memory_get",
        "web_search",
        "team_tasks"
      ],
      "disabled": ["exec", "browser", "spawn", "delegate"]
    },
    "memory_config": { "enabled": true },
    "other_config": {
      "thinking_level": "medium",
      "self_evolve": false
    }
  }
}
```

**SOUL.md cho Translator:**

```markdown
# Translator

Bạn là dịch giả chuyên nghiệp đa ngôn ngữ.

## Năng lực
- Dịch chính xác giữ nguyên ý nghĩa, tone, và cấu trúc
- Hiểu ngữ cảnh chuyên ngành: tech, pháp lý, y tế, marketing, văn học
- Localization: không dịch word-by-word, adapt cho văn hóa đích

## Quy trình dịch
1. Đọc toàn bộ file nguồn trước khi dịch (hiểu context)
2. Xác định thuật ngữ chuyên ngành → tra `memory_search` xem có glossary không
3. Dịch theo section, giữ nguyên format (heading, list, code block, link)
4. Với thuật ngữ mới: ghi vào memory cho lần sau nhất quán
5. Ghi file output vào team workspace: `translated/{tên-file-gốc}`
6. Complete task với `team_tasks(action="complete")`, kèm tóm tắt thay đổi

## Nguyên tắc dịch
- **Không dịch**: tên riêng, brand, code, URL, email
- **Giữ nguyên format**: markdown heading (#, ##), bullet points, bảng, code block
- **Thuật ngữ nhất quán**: dùng glossary trong memory, không đổi cách dịch giữa chừng
- **Ghi chú**: khi gặp đoạn ambiguous, thêm `<!-- translator-note: ... -->` trong output
- **Tone matching**: formal → formal, casual → casual, technical → technical
```

### 1.3 Reviewer (Member)

Agent review chất lượng: check accuracy, fluency, consistency.

```json
{
  "method": "agents.create",
  "params": {
    "name": "Translation Reviewer",
    "agent_type": "predefined",
    "emoji": "✅",
    "tools_config": {
      "enabled": [
        "read_file", "write_file", "edit",
        "skill_search", "use_skill",
        "memory_search",
        "team_tasks"
      ],
      "disabled": ["exec", "browser", "spawn", "delegate", "web_search"]
    },
    "memory_config": { "enabled": true },
    "other_config": {
      "thinking_level": "medium",
      "self_evolve": false
    }
  }
}
```

**SOUL.md cho Reviewer:**

```markdown
# Translation Reviewer

Bạn là biên tập viên dịch thuật chuyên review chất lượng bản dịch.

## Checklist review
1. **Accuracy**: Bản dịch truyền tải đúng ý nghĩa gốc?
2. **Fluency**: Đọc tự nhiên trong ngôn ngữ đích? Không nghe như dịch máy?
3. **Consistency**: Thuật ngữ nhất quán xuyên suốt?
4. **Format**: Heading, list, code block, link còn nguyên?
5. **Missing**: Có đoạn nào bị bỏ sót?
6. **Over-translation**: Có đoạn không nên dịch mà bị dịch? (tên riêng, code)
7. **Cultural fit**: Localization phù hợp? Ví dụ, idiom đã adapt chưa?

## Output
- Sửa trực tiếp bản dịch (dùng `edit` tool)
- Ghi review report: `review/{tên-file}.review.md` gồm:
  - Score: 1-10
  - Issues found (severity: critical/major/minor)
  - Changes made
  - Glossary updates (nếu có)
- Complete task với score trong result

## Nguyên tắc
- Đọc file gốc + file dịch song song
- Không rewrite toàn bộ — chỉ sửa lỗi thực sự
- Lỗi critical: sai nghĩa, mất đoạn → phải sửa
- Lỗi minor: style preference → ghi note, không sửa
```

---

## Bước 2: Tạo Skills

### 2.1 Skill: dich-van-ban

Skill hướng dẫn dịch văn bản tổng quát.

**Tạo thư mục:**
```
workspace/skills/dich-van-ban/
├── SKILL.md
└── references/
    └── translation-patterns.md
```

**SKILL.md:**

```markdown
---
name: dich-van-ban
description: Dịch văn bản giữa các ngôn ngữ. Dùng khi cần translate, dịch file, dịch tài liệu, localize content.
argument-hint: "[ngôn ngữ nguồn→đích] [domain] [file hoặc text]"
---

# Dịch Văn Bản

> Tất cả tiếng Việt phải viết có dấu.

## Quy trình

1. **Phân tích input**: Xác định ngôn ngữ nguồn, đích, domain, tone
2. **Tra glossary**: `memory_search("glossary {domain}")` — dùng thuật ngữ đã thống nhất
3. **Đọc toàn bộ**: Đọc hết file trước khi dịch để hiểu context
4. **Dịch theo section**: Giữ cấu trúc markdown, không cắt giữa paragraph
5. **Post-process**: Check format, link, code block còn nguyên
6. **Ghi output**: File dịch + glossary updates vào memory

## Nguyên tắc vàng

### KHÔNG dịch
- Tên riêng, brand names
- Code blocks, inline code (` `` `)
- URLs, email addresses
- Tên file, path
- Biến, placeholder: `{name}`, `{{count}}`, `$variable`

### Format preservation
- `# Heading` → giữ nguyên cấp heading
- `- bullet` → giữ nguyên
- `| table |` → giữ nguyên cấu trúc, dịch nội dung ô
- `> blockquote` → giữ nguyên
- `[link text](url)` → dịch link text, giữ nguyên url
- `![alt](image)` → dịch alt text, giữ nguyên image path

### Localization (không chỉ translate)
- Idiom/thành ngữ: tìm tương đương văn hóa đích, không dịch literal
- Số liệu: giữ nguyên (không convert đơn vị trừ khi user yêu cầu)
- Ngày tháng: adapt format nếu cần (MM/DD/YYYY → DD/MM/YYYY)
- Ví dụ: adapt cho phù hợp văn hóa đích khi có thể

## References
Chi tiết patterns: [references/translation-patterns.md](references/translation-patterns.md)

$ARGUMENTS
```

**references/translation-patterns.md:**

```markdown
# Translation Patterns

## EN → VI Common Patterns

### Technical terms
| English | Vietnamese | Note |
|---------|-----------|------|
| API | API | Giữ nguyên |
| endpoint | endpoint | Giữ nguyên trong tech context |
| deploy | deploy / triển khai | Tùy context |
| database | cơ sở dữ liệu / database | Formal → CSDL, casual → database |
| cache | cache / bộ nhớ đệm | Tech → cache, docs → bộ nhớ đệm |
| bug | lỗi / bug | Tùy context |
| feature | tính năng | |
| repository | kho lưu trữ / repo | |
| pull request | pull request / PR | Giữ nguyên |
| commit | commit | Giữ nguyên |

### Marketing terms
| English | Vietnamese |
|---------|-----------|
| call to action | lời kêu gọi hành động |
| landing page | trang đích |
| conversion rate | tỷ lệ chuyển đổi |
| user experience | trải nghiệm người dùng |
| brand awareness | nhận diện thương hiệu |

### Legal terms
| English | Vietnamese |
|---------|-----------|
| terms of service | điều khoản dịch vụ |
| privacy policy | chính sách bảo mật |
| liability | trách nhiệm pháp lý |
| indemnify | bồi thường |
| jurisdiction | thẩm quyền xét xử |

## Tone Mapping

| EN Tone | VI Equivalent | Xưng hô |
|---------|--------------|---------|
| Formal/Professional | Trang trọng | chúng tôi / quý khách |
| Casual/Friendly | Thân thiện | mình / bạn |
| Technical | Kỹ thuật | (không xưng hô, viết khách quan) |
| Marketing | Quảng cáo | bạn / chúng tôi |

## VI → EN Common Issues
- Avoid translating "anh/chị/em" literally → use "you"
- Vietnamese passive voice ("được/bị") → choose active when natural in EN
- Classifier words (cái, con, chiếc) → omit in English
- Reduplication (từ láy: xinh xinh, đẹp đẹp) → use adverbs or adjectives
```

### 2.2 Skill: dich-ui

Skill chuyên dịch file i18n (JSON locale files, .po, .xliff).

```markdown
---
name: dich-ui
description: Dịch file i18n cho ứng dụng. Dùng khi cần dịch JSON locale, .po file, .xliff, UI strings, app translation.
argument-hint: "[format] [ngôn ngữ đích] [file]"
---

# Dịch UI / i18n Files

## Hỗ trợ formats
- **JSON** (React i18next, Vue i18n): `{"key": "value"}`
- **Nested JSON**: `{"section": {"key": "value"}}`
- **PO files** (gettext): `msgid` → `msgstr`
- **XLIFF**: XML-based translation format

## Quy tắc đặc biệt cho UI strings

1. **Ngắn gọn**: UI string phải ngắn — button, label, tooltip không dài dòng
2. **Placeholder giữ nguyên**: `{name}`, `{{count}}`, `%s`, `%d` → KHÔNG dịch
3. **Pluralization**: Adapt theo quy tắc ngôn ngữ đích
4. **Context key**: Đọc key name để hiểu context (vd: `button.submit` → nút, không phải heading)
5. **Consistency**: Cùng 1 concept → cùng 1 cách dịch xuyên suốt file

## Quy trình

1. Đọc file nguồn, xác định format (JSON/PO/XLIFF)
2. Tra memory: `memory_search("glossary ui {app-name}")`
3. Dịch value, giữ nguyên key và cấu trúc
4. Validate: JSON phải parse được, PO phải đúng format
5. Output: cùng tên file, thay locale code (vd: `en.json` → `vi.json`)

## Ví dụ

**Input (en.json):**
```json
{
  "nav.home": "Home",
  "nav.settings": "Settings",
  "button.save": "Save changes",
  "button.cancel": "Cancel",
  "message.welcome": "Welcome back, {name}!",
  "message.items": "{count, plural, one {# item} other {# items}}"
}
```

**Output (vi.json):**
```json
{
  "nav.home": "Trang chủ",
  "nav.settings": "Cài đặt",
  "button.save": "Lưu thay đổi",
  "button.cancel": "Hủy",
  "message.welcome": "Chào mừng trở lại, {name}!",
  "message.items": "{count, plural, other {# mục}}"
}
```

$ARGUMENTS
```

### 2.3 Skill: glossary-manager

Skill quản lý thuật ngữ (glossary) để đảm bảo nhất quán.

```markdown
---
name: glossary-manager
description: Quản lý glossary thuật ngữ dịch thuật. Dùng khi cần tạo, cập nhật, tra cứu bảng thuật ngữ, terminology management.
argument-hint: "[action: create/update/lookup] [domain] [terms]"
---

# Quản Lý Glossary

## Actions

### Tạo glossary mới
1. Hỏi: domain (tech, legal, medical, marketing...), ngôn ngữ cặp
2. Tạo file: `memory/glossary-{domain}.md`
3. Format bảng: | Source | Target | Note | Context |

### Tra cứu
1. `memory_search("glossary {domain} {term}")`
2. Trả về bản dịch đã thống nhất + note

### Cập nhật
1. Đọc glossary hiện có: `read_file("memory/glossary-{domain}.md")`
2. Thêm/sửa entry
3. Ghi lại: `write_file("memory/glossary-{domain}.md", ...)`

## Format glossary

```markdown
# Glossary: {Domain}
Ngôn ngữ: {source} → {target}
Cập nhật: {date}

| Source | Target | Note | Context |
|--------|--------|------|---------|
| API | API | Giữ nguyên | Tech |
| deploy | triển khai | Formal context | DevOps |
| deploy | deploy | Casual/internal | Chat |
```

## Nguyên tắc
- Mỗi domain 1 glossary file
- Khi Translator gặp thuật ngữ mới → cập nhật glossary
- Khi Reviewer sửa thuật ngữ → cập nhật glossary + note lý do
- Glossary là source of truth — override sở thích cá nhân

$ARGUMENTS
```

### 2.4 Skill: review-ban-dich

Skill hướng dẫn review bản dịch.

```markdown
---
name: review-ban-dich
description: Review chất lượng bản dịch. Dùng khi cần kiểm tra, đánh giá, QA bản dịch, translation review.
argument-hint: "[file gốc] [file dịch] [focus area]"
---

# Review Bản Dịch

## Checklist (theo thứ tự ưu tiên)

### 1. Accuracy (Chính xác)
- [ ] Mọi câu trong bản gốc đều có trong bản dịch
- [ ] Không dịch sai nghĩa, thêm/bớt ý
- [ ] Số liệu, tên riêng chính xác
- [ ] Placeholder (`{name}`, `{{count}}`) còn nguyên

### 2. Fluency (Tự nhiên)
- [ ] Đọc như văn bản gốc viết bằng ngôn ngữ đích
- [ ] Không có cấu trúc câu lạ (dịch word-by-word)
- [ ] Ngữ pháp đúng
- [ ] Từ ngữ phù hợp ngữ cảnh

### 3. Consistency (Nhất quán)
- [ ] Thuật ngữ giống nhau xuyên suốt
- [ ] Tone/giọng văn đồng nhất
- [ ] Xưng hô nhất quán

### 4. Format (Định dạng)
- [ ] Heading cấp đúng
- [ ] Code block, inline code nguyên vẹn
- [ ] Link hoạt động, URL không đổi
- [ ] Bảng đúng cấu trúc

### 5. Localization (Bản địa hóa)
- [ ] Idiom/thành ngữ adapt phù hợp
- [ ] Ngày tháng đúng format văn hóa đích
- [ ] Ví dụ relevant cho đối tượng đích

## Scoring

| Score | Meaning |
|-------|---------|
| 9-10 | Xuất sắc — ready to publish |
| 7-8 | Tốt — vài lỗi minor |
| 5-6 | Trung bình — cần sửa nhiều |
| 3-4 | Yếu — cần dịch lại phần lớn |
| 1-2 | Không đạt — dịch lại toàn bộ |

## Output format

```markdown
# Review Report: {filename}

## Score: X/10

## Issues Found
### Critical
- Line X: "..." → nên dịch "..." (lý do)

### Major
- Line X: thuật ngữ không nhất quán: "..." vs "..."

### Minor
- Line X: style preference — có thể viết "..." tự nhiên hơn

## Changes Made
- [x] Fixed: ... (line X)
- [x] Fixed: ... (line Y)
- [ ] Noted (not fixed): ... (line Z) — minor, style preference

## Glossary Updates
| Term | Old | New | Reason |
|------|-----|-----|--------|
```

$ARGUMENTS
```

---

## Bước 3: Tạo Team

```json
{
  "method": "teams.create",
  "params": {
    "name": "Translation Team",
    "lead": "translation-manager",
    "members": ["translator", "translation-reviewer"],
    "description": "Pipeline dịch thuật: Manager phân task → Translator dịch → Reviewer kiểm tra",
    "settings": {
      "version": 1,
      "workspace_scope": "shared",
      "notifications": {
        "dispatched": true,
        "progress": true,
        "completed": true,
        "failed": true
      }
    }
  }
}
```

**Workspace scope `"shared"`** — 3 agent cùng thấy files:

```
teams/{teamId}/shared/
├── source/          ← File gốc user gửi
├── translated/      ← Bản dịch từ Translator
├── review/          ← Review report từ Reviewer
├── final/           ← Bản cuối sau review
└── attachments/     ← Auto-copy media từ user
```

---

## Bước 4: Cấu Hình AGENTS.md

Mỗi agent cần biết đồng đội là ai. Khi agent được thêm vào team, GoClaw tự inject `TEAM.md`. Nhưng bạn cũng nên set AGENTS.md rõ ràng cho Translation Manager:

```markdown
# Team Members

## Translator
- Dịch giả chuyên nghiệp đa ngôn ngữ
- Dịch file, văn bản, UI strings
- Giao việc: tạo task qua `team_tasks(action="create", subject="...", assignee="translator")`
- Output: file dịch trong `translated/`

## Translation Reviewer
- Biên tập viên dịch thuật, chuyên review chất lượng
- Check accuracy, fluency, consistency, format
- Giao việc: tạo task qua `team_tasks(action="create", subject="...", assignee="translation-reviewer")`
- Output: review report trong `review/`, file dịch đã sửa
```

---

## Bước 5: Kết Nối Kênh

Kết nối Translation Manager vào kênh bạn muốn dùng:

```json
{
  "method": "channels.create",
  "params": {
    "name": "telegram-translate",
    "channel_type": "telegram",
    "agent_id": "uuid-of-translation-manager",
    "credentials": { "bot_token": "YOUR_BOT_TOKEN" },
    "config": { "dm_policy": "pairing" }
  }
}
```

Hoặc dùng Web UI — chat trực tiếp qua WebSocket.

---

## Bước 6: Sử Dụng — Các Kịch Bản Thực Tế

### Kịch bản 1: Dịch file Markdown EN→VI

**User chat với Translation Manager qua Telegram/Web UI:**

> Dịch file README.md này sang tiếng Việt, tone kỹ thuật
> *(đính kèm file)*

**Pipeline tự động chạy:**

1. **Manager** nhận file + yêu cầu
   - `read_document` hoặc `read_file` đọc nội dung
   - Phân tích: EN→VI, domain tech, tone technical
   - Copy file vào team workspace: `source/README.md`
   - Tạo task:
     ```
     team_tasks(
       action="create",
       subject="Dịch README.md EN→VI",
       description="File: source/README.md\nNgôn ngữ: EN→VI\nDomain: technical\nTone: kỹ thuật, khách quan\nLưu ý: giữ nguyên code block, link, tên package",
       assignee="translator"
     )
     ```
   - Tạo task review (blocked):
     ```
     team_tasks(
       action="create",
       subject="Review bản dịch README.md",
       description="File gốc: source/README.md\nFile dịch: translated/README.md\nCheck: accuracy, format, thuật ngữ tech",
       assignee="translation-reviewer",
       blocked_by=["task-id-dich"]
     )
     ```

2. **Translator** nhận task
   - `read_file("source/README.md")` — đọc file gốc
   - `memory_search("glossary tech")` — tra thuật ngữ
   - `use_skill("dich-van-ban")` → `read_file` skill → dịch theo quy trình
   - `write_file("translated/README.md", content, deliver=false)`
   - Thuật ngữ mới → `write_file("memory/glossary-tech.md", ..., append=true)`
   - `team_tasks(action="complete", result="Đã dịch README.md EN→VI. 3 thuật ngữ mới thêm vào glossary.")`

3. **Reviewer** tự unblock, nhận task
   - `read_file("source/README.md")` — đọc gốc
   - `read_file("translated/README.md")` — đọc bản dịch
   - `use_skill("review-ban-dich")` → chạy checklist
   - Sửa lỗi: `edit("translated/README.md", old="...", new="...")`
   - `write_file("review/README.md.review.md", report)`
   - `team_tasks(action="complete", result="Score: 8/10. 2 lỗi minor đã sửa.")`

4. **Manager** nhận kết quả
   - Đọc bản dịch đã review: `read_file("translated/README.md")`
   - Copy vào final: `write_file("final/README.vi.md", ...)`
   - Gửi cho user: `message(action="send", message="MEDIA:final/README.vi.md")`
   - Tóm tắt: "Dịch xong. Score review: 8/10. File đính kèm."

### Kịch bản 2: Dịch file i18n (JSON locale)

> Dịch file en.json sang vi.json và zh.json cho app

**Manager:**
- Nhận file, phát hiện format JSON i18n
- Tạo 2 task song song (không blocked):
  - "Dịch en.json → vi.json" → Translator
  - "Dịch en.json → zh.json" → Translator (dùng `spawn` để clone)
- Tạo 2 task review (mỗi cái blocked by task dịch tương ứng)

**Translator:**
- `use_skill("dich-ui")` — skill chuyên i18n
- Validate JSON output parseable
- Giữ nguyên key, placeholder

### Kịch bản 3: Dịch hàng loạt (batch)

> Dịch tất cả .md files trong thư mục docs/ sang tiếng Việt

**Manager:**
- `list_files("docs/")` → liệt kê files
- Dùng `spawn` tạo subagent cho chính mình để batch tạo tasks:
  ```
  spawn(
    task="Tạo task dịch cho mỗi file: docs/guide.md, docs/api.md, docs/faq.md. Assignee: translator. Mỗi file 1 task riêng.",
    mode="async"
  )
  ```
- Hoặc tạo lần lượt bằng loop team_tasks
- Mỗi file → 1 task Dịch + 1 task Review (blocked)

### Kịch bản 4: Xây dựng glossary trước khi dịch

> Trước khi dịch tài liệu pháp lý này, tạo glossary thuật ngữ trước

**Manager:**
- Tạo task cho Translator:
  ```
  team_tasks(
    action="create",
    subject="Tạo glossary pháp lý EN→VI",
    description="Đọc file source/contract.md, trích xuất tất cả thuật ngữ pháp lý, tạo glossary. Dùng skill glossary-manager.",
    assignee="translator"
  )
  ```
- Translator tạo `memory/glossary-legal.md`
- Sau đó Manager tạo task dịch (blocked by task glossary)
- Translator dịch với glossary đã có → nhất quán

---

## Bước 7: Memory — Bộ Nhớ Xuyên Session

### Glossary trong Memory

Translator lưu glossary vào memory files. Lần dịch sau, agent tự tra:

```
memory/
├── glossary-tech.md       ← Thuật ngữ công nghệ
├── glossary-legal.md      ← Thuật ngữ pháp lý
├── glossary-marketing.md  ← Thuật ngữ marketing
└── translation-notes.md   ← Ghi chú chung (style decisions, user preferences)
```

**Tự động nhất quán:** Khi Translator gặp "deploy" lần thứ 2, `memory_search("glossary tech deploy")` trả về "triển khai" — không đoán lại.

### Review Feedback Loop

Reviewer sửa thuật ngữ → cập nhật glossary → Translator dùng bản mới cho lần sau:

```
Reviewer sửa: "database" từ "cơ sở dữ liệu" → "CSDL" (ngắn hơn cho UI)
→ Cập nhật memory/glossary-tech.md
→ Lần dịch sau, Translator tra glossary → dùng "CSDL"
```

---

## Bước 8: Mở Rộng

### Thêm ngôn ngữ

Thêm Translator chuyên ngôn ngữ cụ thể:

```json
{
  "name": "Chinese Translator",
  "emoji": "🇨🇳"
}
```

Thêm vào team → Manager biết delegate cho đúng agent theo ngôn ngữ đích.

### Thêm Cron — Dịch tự động

```json
{
  "method": "cron.create",
  "params": {
    "name": "auto-translate-changelog",
    "agent_id": "uuid-of-translation-manager",
    "schedule": { "kind": "cron", "expr": "0 9 * * 1", "tz": "Asia/Ho_Chi_Minh" },
    "payload": {
      "kind": "agent",
      "message": "Check file CHANGELOG.md, nếu có entry mới chưa dịch → tạo task dịch sang VI và ZH."
    }
  }
}
```

Mỗi sáng thứ 2, Manager tự check changelog → dịch tự động → gửi qua Slack/Telegram.

### Thêm MCP Server

Kết nối Google Translate API hoặc DeepL qua MCP để Translator dùng như tool tham khảo (không thay thế, chỉ reference):

```json
{
  "mcp_servers": {
    "deepl": {
      "command": "npx",
      "args": ["deepl-mcp-server"],
      "env": { "DEEPL_API_KEY": "..." }
    }
  }
}
```

Translator có thêm `deepl_translate` tool → dùng làm bản nháp, rồi localize thủ công.

---

## Tóm Tắt Setup

| Component | Chi tiết |
|---|---|
| **Agents** | 3: Manager (lead), Translator (member), Reviewer (member) |
| **Skills** | 4: `dich-van-ban`, `dich-ui`, `glossary-manager`, `review-ban-dich` |
| **Team** | 1: "Translation Team", workspace shared |
| **Memory** | Glossary files per domain, translation notes |
| **Workflow** | Manager → Task Dịch → Task Review (blocked) → Final output |
| **Channel** | Telegram/Web UI/Slack — kết nối với Manager |

**Chi phí ước tính per file:**
- Manager: ~2K tokens (phân tích + tạo task + tổng hợp)
- Translator: ~10-50K tokens (đọc + dịch, phụ thuộc độ dài file)
- Reviewer: ~5-20K tokens (đọc 2 file + review)
- Tổng: ~20-70K tokens/file (với Claude Sonnet ≈ $0.06-0.21/file)
