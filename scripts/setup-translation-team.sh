#!/usr/bin/env bash
#
# GoClaw Translation Pipeline — Full Setup Script
#
# Creates 3 agents, 4 skills, 1 team for a complete
# translation & localization workflow.
#
# Prerequisites:
#   - GoClaw gateway running (default: http://localhost:8080)
#   - zip command available
#   - python3 or jq (for JSON escaping in file content)
#
# Usage:
#   export GOCLAW_TOKEN="your-gateway-token"
#   export GOCLAW_USER_ID="system"  # or your user ID
#   bash scripts/setup-translation-team.sh
#
set -euo pipefail

# ─────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────
BASE_URL="${GOCLAW_URL:-http://localhost:18790}"
TOKEN="${GOCLAW_TOKEN:?Set GOCLAW_TOKEN}"
USER_ID="${GOCLAW_USER_ID:-system}"
AUTH_HEADERS=(
  -H "Authorization: Bearer $TOKEN"
  -H "X-GoClaw-User-Id: $USER_ID"
  -H "Content-Type: application/json"
)

SKILL_DIR=$(mktemp -d)
trap 'rm -rf "$SKILL_DIR"' EXIT

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[x]${NC} $*" >&2; }
info() { echo -e "${CYAN}[i]${NC} $*"; }

# ─────────────────────────────────────────────
# Helper: HTTP call with error checking
# ─────────────────────────────────────────────
api() {
  local method="$1" path="$2"
  shift 2
  local url="${BASE_URL}${path}"
  local resp code body
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" "${AUTH_HEADERS[@]}" "$@" 2>&1) || true
  code=$(echo "$resp" | tail -1)
  body=$(echo "$resp" | sed '$d')

  case "$code" in
    000)
      err "Cannot connect to $BASE_URL — is the gateway running?"
      [[ -n "$body" ]] && err "$body"
      exit 1
      ;;
    2[0-9][0-9])
      echo "$body"
      ;;
    409)
      warn "Already exists (409), skipping..."
      echo "$body"
      ;;
    *)
      err "HTTP $code from $method $path"
      [[ -n "$body" ]] && err "$body"
      exit 1
      ;;
  esac
}

# Pre-flight: check gateway is reachable
log "Checking gateway at $BASE_URL..."
if ! curl -sf "${BASE_URL}/health" > /dev/null 2>&1; then
  err "Gateway not reachable at $BASE_URL"
  err "Start with: ./goclaw  (or set GOCLAW_URL)"
  exit 1
fi
log "Gateway OK"

# ─────────────────────────────────────────────
# Helper: set agent context file via REST API
#   PUT /v1/agents/{id}/files/{fileName}
# ─────────────────────────────────────────────
set_agent_file() {
  local agent_key="$1" file_name="$2" content="$3"
  local escaped
  escaped=$(printf '%s' "$content" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))' 2>/dev/null \
    || printf '%s' "$content" | jq -Rs '.' 2>/dev/null \
    || { err "Need python3 or jq to escape file content"; return 1; })

  api PUT "/v1/agents/${agent_key}/files/${file_name}" \
    -d "{\"content\":$escaped}" > /dev/null
}

# ═══════════════════════════════════════════════
#  STEP 1: Create Agents
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 1: Creating 3 Agents"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# --- Agent 1: Translation Manager (Lead) ---
log "Creating agent: Translation Manager..."
MANAGER_RESP=$(api POST /v1/agents -d "$(cat <<'AGENT_JSON'
{
  "agent_key": "translation-manager",
  "display_name": "Translation Manager",
  "agent_type": "predefined",
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
  "memory_config": {"enabled": true},
  "other_config": {
    "thinking_level": "low",
    "self_evolve": false,
    "max_tokens": 8192
  }
}
AGENT_JSON
)")
MANAGER_ID=$(echo "$MANAGER_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "Manager ID: ${MANAGER_ID:-unknown}"

# --- Agent 2: Translator (Member) ---
log "Creating agent: Translator..."
TRANSLATOR_RESP=$(api POST /v1/agents -d "$(cat <<'AGENT_JSON'
{
  "agent_key": "translator",
  "display_name": "Translator",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": [
      "read_file", "write_file", "edit",
      "read_document",
      "skill_search", "use_skill",
      "memory_search", "memory_get",
      "web_search",
      "team_tasks"
    ],
    "disabled": ["exec", "browser", "spawn"]
  },
  "memory_config": {"enabled": true},
  "other_config": {
    "thinking_level": "medium",
    "self_evolve": false,
    "max_tokens": 16384
  }
}
AGENT_JSON
)")
TRANSLATOR_ID=$(echo "$TRANSLATOR_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "Translator ID: ${TRANSLATOR_ID:-unknown}"

# --- Agent 3: Reviewer (Member) ---
log "Creating agent: Translation Reviewer..."
REVIEWER_RESP=$(api POST /v1/agents -d "$(cat <<'AGENT_JSON'
{
  "agent_key": "translation-reviewer",
  "display_name": "Translation Reviewer",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": [
      "read_file", "write_file", "edit",
      "skill_search", "use_skill",
      "memory_search",
      "team_tasks"
    ],
    "disabled": ["exec", "browser", "spawn", "web_search"]
  },
  "memory_config": {"enabled": true},
  "other_config": {
    "thinking_level": "medium",
    "self_evolve": false,
    "max_tokens": 8192
  }
}
AGENT_JSON
)")
REVIEWER_ID=$(echo "$REVIEWER_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "Reviewer ID: ${REVIEWER_ID:-unknown}"

# ═══════════════════════════════════════════════
#  STEP 2: Set SOUL.md for each agent
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 2: Setting SOUL.md (agent personalities)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# --- SOUL.md: Translation Manager ---
log "Setting SOUL.md for Translation Manager..."
set_agent_file "translation-manager" "SOUL.md" "$(cat <<'SOUL'
# Translation Manager

Bạn là project manager chuyên quản lý pipeline dịch thuật.

## Vai trò
- Nhận yêu cầu dịch từ user (file, text, hoặc paste)
- Phân tích: ngôn ngữ nguồn, ngôn ngữ đích, domain chuyên ngành, tone
- Tách file lớn thành chunks hợp lý (theo heading/section, không cắt giữa câu)
- Tạo task cho Translator với context đầy đủ
- Tạo task Review (blocked by task Dịch) cho Reviewer
- Tổng hợp bản dịch hoàn chỉnh, gửi lại user

## Quy trình
1. User gửi file/text → phân tích ngôn ngữ nguồn, đích, domain
2. Nếu chưa rõ → hỏi: ngôn ngữ đích? tone (formal/casual)? domain?
3. Copy file vào team workspace: `source/{tên-file}`
4. Tạo task "Dịch {file}" → assign "translator"
5. Tạo task "Review {file}" → assign "translation-reviewer", blocked_by task dịch
6. Theo dõi progress: `team_tasks(action="list")`
7. Khi review xong → đọc bản cuối → gửi user qua `message(MEDIA:...)`

## Nguyên tắc
- Luôn ghi rõ trong description: ngôn ngữ, domain, tone, thuật ngữ đặc biệt
- File nhỏ (<3000 words) → 1 task. File lớn → tách theo section headings
- Theo dõi progress, không để task stale
- Gửi file dịch xong cho user, kèm review score
SOUL
)"

# --- SOUL.md: Translator ---
log "Setting SOUL.md for Translator..."
set_agent_file "translator" "SOUL.md" "$(cat <<'SOUL'
# Translator

Bạn là dịch giả chuyên nghiệp đa ngôn ngữ.

## Năng lực
- Dịch chính xác giữ nguyên ý nghĩa, tone, và cấu trúc
- Hiểu ngữ cảnh chuyên ngành: tech, pháp lý, y tế, marketing, văn học
- Localization: không dịch word-by-word, adapt cho văn hóa đích

## Quy trình
1. Đọc toàn bộ file nguồn trước khi dịch (hiểu context)
2. Tra `memory_search("glossary {domain}")` — dùng thuật ngữ đã thống nhất
3. Dùng `skill_search` tìm skill dịch phù hợp (dich-van-ban hoặc dich-ui)
4. Dịch theo section, giữ nguyên format markdown
5. Thuật ngữ mới → ghi vào memory/glossary-{domain}.md
6. Output → `translated/{tên-file}` trong team workspace
7. Complete task: `team_tasks(action="complete", result="tóm tắt")`

## Nguyên tắc dịch
- KHÔNG dịch: tên riêng, brand, code, URL, email, placeholder ({name}, {{count}})
- GIỮ NGUYÊN: markdown heading, bullet, bảng, code block, link URL
- Thuật ngữ nhất quán: dùng glossary, không đổi giữa chừng
- Đoạn ambiguous: thêm <!-- translator-note: ... --> trong output
- Tone matching: formal → formal, casual → casual
SOUL
)"

# --- SOUL.md: Reviewer ---
log "Setting SOUL.md for Translation Reviewer..."
set_agent_file "translation-reviewer" "SOUL.md" "$(cat <<'SOUL'
# Translation Reviewer

Bạn là biên tập viên dịch thuật chuyên review chất lượng bản dịch.

## Checklist review (theo thứ tự ưu tiên)
1. **Accuracy**: Bản dịch đúng ý nghĩa gốc? Không sai, không thiếu?
2. **Fluency**: Đọc tự nhiên? Không nghe như dịch máy?
3. **Consistency**: Thuật ngữ nhất quán xuyên suốt?
4. **Format**: Heading, list, code block, link còn nguyên?
5. **Completeness**: Không bỏ sót đoạn nào?
6. **Localization**: Idiom, ví dụ adapt cho văn hóa đích?

## Quy trình
1. Đọc file gốc: `read_file("source/{file}")`
2. Đọc bản dịch: `read_file("translated/{file}")`
3. Dùng `skill_search("review dịch")` → load skill review-ban-dich
4. So sánh từng section gốc vs dịch
5. Sửa lỗi trực tiếp: `edit("translated/{file}", old="...", new="...")`
6. Ghi review report: `write_file("review/{file}.review.md", ...)`
7. Cập nhật glossary nếu cần
8. Complete task: `team_tasks(action="complete", result="Score: X/10. N lỗi đã sửa.")`

## Scoring
- 9-10: Xuất sắc, ready to publish
- 7-8: Tốt, vài lỗi minor
- 5-6: Trung bình, cần sửa nhiều
- 3-4: Yếu, cần dịch lại phần lớn
- 1-2: Không đạt

## Nguyên tắc
- Đọc song song gốc + dịch
- Chỉ sửa lỗi thực sự, không rewrite style preference
- Lỗi critical (sai nghĩa, mất đoạn) → phải sửa
- Lỗi minor (style) → ghi note, không sửa
SOUL
)"

# --- AGENTS.md for Manager ---
log "Setting AGENTS.md for Translation Manager..."
set_agent_file "translation-manager" "AGENTS.md" "$(cat <<'AGENTS'
# Team Members

## translator
- Dịch giả chuyên nghiệp đa ngôn ngữ
- Dịch file, văn bản, UI strings (JSON i18n, PO, XLIFF)
- Giỏi: technical, legal, marketing, literary translation
- Giao việc: `team_tasks(action="create", subject="Dịch ...", assignee="translator")`
- Output: `translated/{filename}` trong team workspace

## translation-reviewer
- Biên tập viên dịch thuật chuyên review chất lượng
- Check accuracy, fluency, consistency, format
- Giao việc: `team_tasks(action="create", subject="Review ...", assignee="translation-reviewer")`
- Output: sửa file dịch trực tiếp + review report trong `review/`
AGENTS
)"

# ═══════════════════════════════════════════════
#  STEP 3: Create and upload Skills (ZIP)
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 3: Creating & uploading 4 Skills"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ── Skill 1: dich-van-ban ──
log "Building skill: dich-van-ban..."
mkdir -p "$SKILL_DIR/dich-van-ban/references"

cat > "$SKILL_DIR/dich-van-ban/SKILL.md" <<'EOF'
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
6. **Ghi output**: File dịch vào `translated/` + glossary updates vào memory

## Nguyên tắc

### KHÔNG dịch
- Tên riêng, brand names
- Code blocks, inline code
- URLs, email addresses
- Tên file, path
- Placeholder: `{name}`, `{{count}}`, `$variable`

### Format preservation
- `# Heading` → giữ nguyên cấp heading
- `- bullet` → giữ nguyên
- `| table |` → giữ cấu trúc, dịch nội dung ô
- `[link text](url)` → dịch link text, giữ nguyên url
- `![alt](image)` → dịch alt text, giữ nguyên image path

### Localization
- Idiom → tìm tương đương văn hóa đích, không dịch literal
- Số liệu → giữ nguyên
- Ngày tháng → adapt format nếu cần (MM/DD → DD/MM)

Tham khảo chi tiết: [references/translation-patterns.md](references/translation-patterns.md)

$ARGUMENTS
EOF

cat > "$SKILL_DIR/dich-van-ban/references/translation-patterns.md" <<'EOF'
# Translation Patterns

## EN → VI Common Patterns

### Technical terms
| English | Vietnamese | Note |
|---------|-----------|------|
| API | API | Giữ nguyên |
| endpoint | endpoint | Giữ nguyên trong tech context |
| deploy | triển khai / deploy | Formal → triển khai, casual → deploy |
| database | cơ sở dữ liệu / database | Formal → CSDL, casual → database |
| cache | bộ nhớ đệm / cache | Tech docs → cache, user docs → bộ nhớ đệm |
| feature | tính năng | |
| pull request | pull request / PR | Giữ nguyên |
| commit | commit | Giữ nguyên |
| repository | kho lưu trữ / repo | |

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

## Tone Mapping
| EN Tone | VI Equivalent | Xưng hô |
|---------|--------------|---------|
| Formal | Trang trọng | chúng tôi / quý khách |
| Casual | Thân thiện | mình / bạn |
| Technical | Kỹ thuật | không xưng hô, viết khách quan |
| Marketing | Quảng cáo | bạn / chúng tôi |
EOF

(cd "$SKILL_DIR" && zip -r dich-van-ban.zip dich-van-ban/)
log "Uploading skill: dich-van-ban..."
SKILL1_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/dich-van-ban.zip")
SKILL1_CODE=$(echo "$SKILL1_RESP" | tail -1)
SKILL1_BODY=$(echo "$SKILL1_RESP" | sed '$d')
SKILL1_ID=$(echo "$SKILL1_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "dich-van-ban ID: ${SKILL1_ID:-upload_failed ($SKILL1_CODE)}"

# ── Skill 2: dich-ui ──
log "Building skill: dich-ui..."
mkdir -p "$SKILL_DIR/dich-ui"

cat > "$SKILL_DIR/dich-ui/SKILL.md" <<'EOF'
---
name: dich-ui
description: Dịch file i18n cho ứng dụng. Dùng khi cần dịch JSON locale, .po file, .xliff, UI strings, app translation.
argument-hint: "[format] [ngôn ngữ đích] [file]"
---

# Dịch UI / i18n Files

## Hỗ trợ formats
- **JSON** (React i18next, Vue i18n): `{"key": "value"}`
- **Nested JSON**: `{"section": {"key": "value"}}`
- **PO files** (gettext): `msgid` / `msgstr`
- **XLIFF**: XML-based translation format

## Quy tắc đặc biệt

1. **Ngắn gọn**: UI string phải ngắn — button, label, tooltip không dài dòng
2. **Placeholder giữ nguyên**: `{name}`, `{{count}}`, `%s`, `%d` → KHÔNG dịch
3. **Pluralization**: Adapt theo quy tắc ngôn ngữ đích
4. **Context từ key**: Đọc key name để hiểu (vd: `button.submit` → nút, không phải heading)
5. **Consistency**: Cùng concept → cùng cách dịch xuyên suốt file

## Quy trình

1. Đọc file nguồn → xác định format (JSON/PO/XLIFF)
2. Tra memory: `memory_search("glossary ui")`
3. Dịch value, giữ nguyên key và cấu trúc
4. Validate: JSON phải parse được, PO đúng format
5. Output: cùng tên file, thay locale code (vd: `en.json` → `vi.json`)

## Ví dụ

Input (en.json):
```json
{
  "nav.home": "Home",
  "nav.settings": "Settings",
  "button.save": "Save changes",
  "message.welcome": "Welcome back, {name}!"
}
```

Output (vi.json):
```json
{
  "nav.home": "Trang chủ",
  "nav.settings": "Cài đặt",
  "button.save": "Lưu thay đổi",
  "message.welcome": "Chào mừng trở lại, {name}!"
}
```

$ARGUMENTS
EOF

(cd "$SKILL_DIR" && zip -r dich-ui.zip dich-ui/)
log "Uploading skill: dich-ui..."
SKILL2_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/dich-ui.zip")
SKILL2_CODE=$(echo "$SKILL2_RESP" | tail -1)
SKILL2_BODY=$(echo "$SKILL2_RESP" | sed '$d')
SKILL2_ID=$(echo "$SKILL2_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "dich-ui ID: ${SKILL2_ID:-upload_failed ($SKILL2_CODE)}"

# ── Skill 3: glossary-manager ──
log "Building skill: glossary-manager..."
mkdir -p "$SKILL_DIR/glossary-manager"

cat > "$SKILL_DIR/glossary-manager/SKILL.md" <<'EOF'
---
name: glossary-manager
description: Quản lý glossary thuật ngữ dịch thuật. Dùng khi cần tạo, cập nhật, tra cứu bảng thuật ngữ, terminology management.
argument-hint: "[action: create/update/lookup] [domain] [terms]"
---

# Quản Lý Glossary

## Actions

### Tạo glossary mới
1. Xác định: domain (tech, legal, medical, marketing...), ngôn ngữ cặp
2. Tạo file: `memory/glossary-{domain}.md`
3. Format: | Source | Target | Note | Context |

### Tra cứu
1. `memory_search("glossary {domain} {term}")`
2. Trả về bản dịch đã thống nhất + note

### Cập nhật
1. Đọc: `read_file("memory/glossary-{domain}.md")`
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
| deploy | triển khai | Formal | DevOps |
```

## Nguyên tắc
- Mỗi domain 1 glossary file riêng
- Translator gặp thuật ngữ mới → cập nhật ngay
- Reviewer sửa thuật ngữ → cập nhật + ghi lý do
- Glossary là source of truth — override sở thích cá nhân

$ARGUMENTS
EOF

(cd "$SKILL_DIR" && zip -r glossary-manager.zip glossary-manager/)
log "Uploading skill: glossary-manager..."
SKILL3_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/glossary-manager.zip")
SKILL3_CODE=$(echo "$SKILL3_RESP" | tail -1)
SKILL3_BODY=$(echo "$SKILL3_RESP" | sed '$d')
SKILL3_ID=$(echo "$SKILL3_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "glossary-manager ID: ${SKILL3_ID:-upload_failed ($SKILL3_CODE)}"

# ── Skill 4: review-ban-dich ──
log "Building skill: review-ban-dich..."
mkdir -p "$SKILL_DIR/review-ban-dich"

cat > "$SKILL_DIR/review-ban-dich/SKILL.md" <<'EOF'
---
name: review-ban-dich
description: Review chất lượng bản dịch. Dùng khi cần kiểm tra, đánh giá, QA bản dịch, translation review, quality assurance.
argument-hint: "[file gốc] [file dịch] [focus area]"
---

# Review Bản Dịch

## Checklist (theo thứ tự ưu tiên)

### 1. Accuracy
- [ ] Mọi câu gốc đều có trong bản dịch
- [ ] Không sai nghĩa, không thêm/bớt ý
- [ ] Số liệu, tên riêng chính xác
- [ ] Placeholder còn nguyên

### 2. Fluency
- [ ] Đọc tự nhiên trong ngôn ngữ đích
- [ ] Không có cấu trúc dịch word-by-word
- [ ] Ngữ pháp đúng

### 3. Consistency
- [ ] Thuật ngữ nhất quán xuyên suốt
- [ ] Tone đồng nhất
- [ ] Xưng hô nhất quán

### 4. Format
- [ ] Heading cấp đúng
- [ ] Code block, inline code nguyên vẹn
- [ ] Link URL không đổi
- [ ] Bảng đúng cấu trúc

### 5. Localization
- [ ] Idiom adapt phù hợp
- [ ] Ngày tháng đúng format

## Scoring
| Score | Meaning |
|-------|---------|
| 9-10 | Xuất sắc — ready to publish |
| 7-8 | Tốt — vài lỗi minor |
| 5-6 | Trung bình — cần sửa nhiều |
| 3-4 | Yếu — cần dịch lại phần lớn |
| 1-2 | Không đạt |

## Output format

```markdown
# Review Report: {filename}
## Score: X/10
## Issues Found
### Critical
- Line X: "..." → nên dịch "..."
### Major
- Thuật ngữ không nhất quán: "..." vs "..."
### Minor
- Style preference (không sửa)
## Changes Made
- [x] Fixed: ...
## Glossary Updates
| Term | Old | New | Reason |
```

$ARGUMENTS
EOF

(cd "$SKILL_DIR" && zip -r review-ban-dich.zip review-ban-dich/)
log "Uploading skill: review-ban-dich..."
SKILL4_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/review-ban-dich.zip")
SKILL4_CODE=$(echo "$SKILL4_RESP" | tail -1)
SKILL4_BODY=$(echo "$SKILL4_RESP" | sed '$d')
SKILL4_ID=$(echo "$SKILL4_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "review-ban-dich ID: ${SKILL4_ID:-upload_failed ($SKILL4_CODE)}"

# ═══════════════════════════════════════════════
#  STEP 4: Grant Skills to Agents
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 4: Granting skills to agents"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

grant_skill() {
  local skill_id="$1" agent_id="$2" skill_name="$3" agent_name="$4"
  if [[ -z "$skill_id" || "$skill_id" == "null" || -z "$agent_id" || "$agent_id" == "unknown" ]]; then
    warn "Skipping grant $skill_name → $agent_name (missing ID)"
    return
  fi
  log "Granting $skill_name → $agent_name..."
  api POST "/v1/skills/${skill_id}/grants/agent" \
    -d "{\"agent_id\":\"${agent_id}\"}" > /dev/null 2>&1 || true
}

# Grant all 4 skills to all 3 agents
SKILL_IDS=("$SKILL1_ID" "$SKILL2_ID" "$SKILL3_ID" "$SKILL4_ID")
SKILL_NAMES=("dich-van-ban" "dich-ui" "glossary-manager" "review-ban-dich")
AGENT_IDS=("$MANAGER_ID" "$TRANSLATOR_ID" "$REVIEWER_ID")
AGENT_NAMES=("manager" "translator" "reviewer")

for i in 0 1 2 3; do
  for j in 0 1 2; do
    grant_skill "${SKILL_IDS[$i]}" "${AGENT_IDS[$j]}" "${SKILL_NAMES[$i]}" "${AGENT_NAMES[$j]}"
  done
done

# ═══════════════════════════════════════════════
#  STEP 5: Create Team (HTTP REST API)
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 5: Creating Translation Team"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

log "Creating team via REST API..."
TEAM_RESP=$(api POST /v1/teams -d "$(cat <<'TEAM_JSON'
{
  "name": "Translation Team",
  "lead": "translation-manager",
  "members": ["translator", "translation-reviewer"],
  "description": "Pipeline dịch thuật: Manager phân task → Translator dịch → Reviewer kiểm tra chất lượng",
  "settings": {
    "version": 1,
    "workspace_scope": "shared",
    "notifications": {
      "dispatched": true,
      "progress": true,
      "completed": true,
      "failed": true,
      "new_task": true
    }
  }
}
TEAM_JSON
)" || true)

if [[ -n "$TEAM_RESP" ]]; then
  TEAM_ID=$(echo "$TEAM_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  info "Team ID: ${TEAM_ID:-check response}"
else
  warn "Failed to create team. Check gateway logs."
fi

# ═══════════════════════════════════════════════
#  DONE
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Setup Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Agents:"
echo "    translation-manager   ${MANAGER_ID:-N/A}"
echo "    translator            ${TRANSLATOR_ID:-N/A}"
echo "    translation-reviewer  ${REVIEWER_ID:-N/A}"
echo ""
echo "  Skills:"
echo "    dich-van-ban          ${SKILL1_ID:-N/A}"
echo "    dich-ui               ${SKILL2_ID:-N/A}"
echo "    glossary-manager      ${SKILL3_ID:-N/A}"
echo "    review-ban-dich       ${SKILL4_ID:-N/A}"
echo ""
echo "  Team: Translation Team  ${TEAM_ID:-create via UI}"
echo ""
echo "  Next steps:"
echo "    1. Open Web UI → chat with 'Translation Manager'"
echo "    2. Send a file and say: 'Dịch file này sang tiếng Việt'"
echo "    3. Watch the pipeline: Manager → Translator → Reviewer → Done"
echo ""
