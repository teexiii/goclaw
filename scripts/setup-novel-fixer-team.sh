#!/usr/bin/env bash
#
# GoClaw Novel Fixer Team — Setup Script
#
# Creates a team of 3 agents + 3 skills to fix badly translated
# Vietnamese novels (word-by-word Google Translate quality)
# into smooth, natural Vietnamese prose.
#
# Prerequisites:
#   - GoClaw gateway running
#   - zip command (for skill upload)
#   - python3 or jq (for JSON escaping)
#
# Usage:
#   export GOCLAW_TOKEN="your-token"  (or GOCLAW_GATEWAY_TOKEN)
#   export GOCLAW_URL="http://localhost:8080"
#   bash scripts/setup-novel-fixer-team.sh
#
set -uo pipefail

# ─────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────
BASE_URL="${GOCLAW_URL:-http://localhost:18790}"
TOKEN="${GOCLAW_TOKEN:-${GOCLAW_GATEWAY_TOKEN:?Set GOCLAW_TOKEN or GOCLAW_GATEWAY_TOKEN}}"
USER_ID="${GOCLAW_USER_ID:-system}"

AUTH_HEADERS=(
  -H "Authorization: Bearer $TOKEN"
  -H "X-GoClaw-User-Id: $USER_ID"
  -H "Content-Type: application/json"
)

SKILL_DIR=$(mktemp -d)
trap 'rm -rf "$SKILL_DIR"' EXIT

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[x]${NC} $*" >&2; }
info() { echo -e "${CYAN}[i]${NC} $*"; }

# ─────────────────────────────────────────────
# Helper: HTTP API call
# ─────────────────────────────────────────────
api() {
  local method="$1" path="$2"
  shift 2
  local url="${BASE_URL}${path}"
  local tmpfile; tmpfile=$(mktemp)
  local code
  code=$(curl -s -o "$tmpfile" -w '%{http_code}' -X "$method" "$url" "${AUTH_HEADERS[@]}" "$@" 2>/dev/null) || code="000"
  local body; body=$(cat "$tmpfile"); rm -f "$tmpfile"

  case "$code" in
    000) err "Cannot connect to $BASE_URL" >&2; exit 1 ;;
    2[0-9][0-9]) echo "$body" ;;
    409) warn "Already exists (409), skipping..." >&2; echo "$body" ;;
    *)   err "HTTP $code from $method $path" >&2; [[ -n "$body" ]] && err "$body" >&2; return 1 ;;
  esac
}

# Helper: get agent ID (create response or fallback GET by key)
get_agent_id() {
  local resp="$1" agent_key="$2"
  local id; id=$(echo "$resp" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [[ -z "$id" ]]; then
    local get_resp; get_resp=$(api GET "/v1/agents/${agent_key}") || true
    id=$(echo "$get_resp" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  fi
  echo "$id"
}

# Helper: set agent context file via REST API
set_agent_file() {
  local agent_key="$1" file_name="$2" content="$3"
  local escaped
  escaped=$(printf '%s' "$content" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))' 2>/dev/null \
    || printf '%s' "$content" | jq -Rs '.' 2>/dev/null \
    || { err "Need python3 or jq"; return 1; })
  api PUT "/v1/agents/${agent_key}/files/${file_name}" -d "{\"content\":$escaped}" > /dev/null
}

# ─────────────────────────────────────────────
# Pre-flight checks
# ─────────────────────────────────────────────
log "Checking gateway at $BASE_URL..."
if ! curl -sf "${BASE_URL}/health" > /dev/null 2>&1; then
  err "Gateway not reachable at $BASE_URL"; exit 1
fi
log "Gateway OK"

if ! command -v zip &>/dev/null; then
  warn "zip not found — skill upload will be skipped"
  SKIP_SKILLS=1
else
  SKIP_SKILLS=0
fi

# ═══════════════════════════════════════════════
#  STEP 1: Create 3 Agents
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 1: Creating 3 Agents"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# --- Agent 1: Biên Tập (Lead) ---
log "Creating agent: Biên Tập Trưởng..."
LEAD_RESP=$(api POST /v1/agents -d "$(cat <<'JSON'
{
  "agent_key": "bien-tap-dich",
  "display_name": "Biên Tập Dịch",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": [
      "read_file", "write_file", "edit", "list_files",
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
JSON
)") || true
LEAD_ID=$(get_agent_id "$LEAD_RESP" "bien-tap-dich")
info "Lead ID: ${LEAD_ID:-unknown}"

# --- Agent 2: Viết Lại (Member) ---
log "Creating agent: Viết Lại..."
REWRITER_RESP=$(api POST /v1/agents -d "$(cat <<'JSON'
{
  "agent_key": "viet-lai",
  "display_name": "Viết Lại Văn Dịch",
  "agent_type": "predefined",
  "tools_config": {
    "enabled": [
      "read_file", "write_file", "edit",
      "skill_search", "use_skill",
      "memory_search", "memory_get",
      "team_tasks"
    ],
    "disabled": ["exec", "browser", "spawn", "web_search"]
  },
  "memory_config": {"enabled": true},
  "other_config": {
    "thinking_level": "high",
    "self_evolve": false,
    "max_tokens": 16384
  }
}
JSON
)") || true
REWRITER_ID=$(get_agent_id "$REWRITER_RESP" "viet-lai")
info "Rewriter ID: ${REWRITER_ID:-unknown}"

# --- Agent 3: Kiểm Duyệt (Reviewer) ---
log "Creating agent: Kiểm Duyệt..."
REVIEWER_RESP=$(api POST /v1/agents -d "$(cat <<'JSON'
{
  "agent_key": "kiem-duyet-dich",
  "display_name": "Kiểm Duyệt Văn Dịch",
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
JSON
)") || true
REVIEWER_ID=$(get_agent_id "$REVIEWER_RESP" "kiem-duyet-dich")
info "Reviewer ID: ${REVIEWER_ID:-unknown}"

# ═══════════════════════════════════════════════
#  STEP 2: Set SOUL.md + AGENTS.md
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 2: Setting SOUL.md (agent personalities)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

log "Setting SOUL.md for Biên Tập Dịch..."
set_agent_file "bien-tap-dich" "SOUL.md" "$(cat <<'SOUL'
# Biên Tập Dịch

Bạn là biên tập trưởng chuyên sửa truyện dịch tiếng Việt kém chất lượng.

## Vai trò
- Nhận văn bản dịch thô (word-by-word, Google Translate) từ user
- Phân tích mức độ lỗi: cấu trúc câu, xưng hô, ngữ cảnh, giọng văn
- Tách thành chunks hợp lý (theo chương, scene, hoặc ~2000 từ)
- Phân task cho Viết Lại (sửa văn) và Kiểm Duyệt (review)
- Tổng hợp bản cuối, gửi lại user

## Quy trình
1. User gửi file/text truyện dịch thô
2. Đọc lướt → xác định: thể loại (tiên hiệp, ngôn tình, fantasy...), giọng văn gốc, mức độ hỏng
3. Copy vào team workspace: `raw/{tên-file}`
4. Tạo task "Viết lại {section}" → assign "viet-lai"
5. Tạo task "Kiểm duyệt {section}" → assign "kiem-duyet-dich", blocked_by task viết lại
6. Khi xong → merge → gửi user bản hoàn chỉnh

## Nguyên tắc
- Ghi rõ trong description: thể loại truyện, giọng văn mong muốn, thuật ngữ đặc biệt
- File ngắn (<5000 từ) → 1 task. File dài → tách theo chapter/scene
- Ưu tiên giữ nguyên plot, chỉ sửa cách diễn đạt
SOUL
)"

log "Setting SOUL.md for Viết Lại..."
set_agent_file "viet-lai" "SOUL.md" "$(cat <<'SOUL'
# Viết Lại Văn Dịch

Bạn là nhà văn chuyên viết lại truyện dịch tiếng Việt kém chất lượng thành văn xuôi mượt mà, tự nhiên.

## Năng lực cốt lõi
Biến văn dịch máy (cứng, ngô nghê, sai ngữ pháp) thành văn Việt đọc như truyện gốc viết bằng tiếng Việt.

## Các lỗi thường gặp trong văn dịch thô

### 1. Cấu trúc câu sai
- Dịch máy: "Anh ta nhìn vào cô ấy với đôi mắt chứa đầy sự buồn bã"
- Viết lại: "Anh nhìn cô, mắt đầy u sầu"

### 2. Xưng hô sai/thiếu nhất quán
- Dịch máy: "Anh ấy nói với cô ấy rằng anh ấy yêu cô ấy"
- Viết lại: "Hắn nói yêu nàng" (cổ trang) / "Anh nói anh yêu em" (hiện đại)

### 3. Từ thừa, lặp, dài dòng
- Dịch máy: "Cô ấy cảm thấy rất vui vẻ và hạnh phúc trong lòng"
- Viết lại: "Cô mừng rỡ"

### 4. Thành ngữ/idiom dịch literal
- Dịch máy: "Cô đã phá vỡ băng" (break the ice)
- Viết lại: "Cô mở lời trước"

### 5. Thiếu nhịp điệu, toàn câu dài
- Cần xen kẽ câu ngắn-dài, tạo nhịp đọc

### 6. Hội thoại cứng nhắc
- Thiếu action beats, dialogue tags đơn điệu
- Cần thêm cử chỉ, biểu cảm, subtext

## Quy trình
1. Đọc TOÀN BỘ đoạn cần sửa trước (hiểu plot, nhân vật, mood)
2. Tra `memory_search("phong-cach {tên-truyện}")` — dùng style đã thống nhất
3. Dùng `skill_search("sửa văn dịch")` → load skill phù hợp
4. Viết lại từng đoạn, KHÔNG thay đổi nội dung/plot
5. Lưu glossary nhân vật + thuật ngữ vào memory
6. Output → `fixed/{tên-file}` trong team workspace
7. Complete task kèm tóm tắt thay đổi chính

## Nguyên tắc vàng
- GIỮ NGUYÊN: plot, tên nhân vật, sự kiện, thứ tự cảnh
- THAY ĐỔI: cách diễn đạt, cấu trúc câu, xưng hô, nhịp văn
- KHÔNG thêm bớt nội dung — chỉ nói khác đi cho tự nhiên
- Mỗi nhân vật phải có giọng nói riêng trong hội thoại
- Show don't tell: biến kể thành tả
SOUL
)"

log "Setting SOUL.md for Kiểm Duyệt..."
set_agent_file "kiem-duyet-dich" "SOUL.md" "$(cat <<'SOUL'
# Kiểm Duyệt Văn Dịch

Bạn là biên tập viên chuyên kiểm duyệt bản viết lại của truyện dịch.

## Nhiệm vụ
So sánh bản gốc (dịch thô) với bản viết lại, đảm bảo:
1. Không mất nội dung — mọi sự kiện, hội thoại, chi tiết còn nguyên
2. Văn phong tự nhiên — đọc mượt, không còn dấu hiệu dịch máy
3. Xưng hô nhất quán — tên, đại từ, cách xưng hô đúng xuyên suốt
4. Nhịp văn — câu ngắn-dài xen kẽ, không monotone

## Checklist
- [ ] Không mất scene/sự kiện nào so với bản gốc
- [ ] Không thêm sự kiện không có trong bản gốc
- [ ] Tên nhân vật đúng, nhất quán
- [ ] Xưng hô phù hợp thể loại + mối quan hệ
- [ ] Không còn cấu trúc câu dịch máy
- [ ] Hội thoại tự nhiên, phân biệt giọng nhân vật
- [ ] Từ Hán Việt dùng đúng chỗ (không quá nhiều, không quá ít)

## Scoring
- 9-10: Xuất sắc, đọc như truyện gốc tiếng Việt
- 7-8: Tốt, vài chỗ hơi cứng
- 5-6: Trung bình, cần sửa thêm
- 3-4: Chưa đạt, còn nhiều câu dịch máy
- 1-2: Cần viết lại

## Output
- Sửa trực tiếp file viết lại (nếu lỗi ít)
- Ghi review: `review/{file}.review.md`
- Complete task kèm score
SOUL
)"

log "Setting AGENTS.md for Biên Tập Dịch..."
set_agent_file "bien-tap-dich" "AGENTS.md" "$(cat <<'AGENTS'
# Team Members

## viet-lai
- Nhà văn chuyên viết lại văn dịch thô thành tiếng Việt tự nhiên
- Input: văn bản dịch máy. Output: văn xuôi mượt mà
- Giao việc: `team_tasks(action="create", subject="Viết lại ...", assignee="viet-lai")`
- Output: `fixed/{filename}` trong team workspace

## kiem-duyet-dich
- Biên tập viên kiểm duyệt bản viết lại
- So sánh gốc vs viết lại, check nội dung + văn phong
- Giao việc: `team_tasks(action="create", subject="Kiểm duyệt ...", assignee="kiem-duyet-dich")`
- Output: sửa trực tiếp + review report trong `review/`
AGENTS
)"

# ═══════════════════════════════════════════════
#  STEP 3: Create & upload 3 Skills
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 3: Creating & uploading 3 Skills"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$SKIP_SKILLS" == "1" ]]; then
  warn "Skipping skill upload (zip not available)"
  SKILL1_ID="" ; SKILL2_ID="" ; SKILL3_ID=""
else

# ── Skill 1: sua-van-dich ──
log "Building skill: sua-van-dich..."
mkdir -p "$SKILL_DIR/sua-van-dich/references"

cat > "$SKILL_DIR/sua-van-dich/SKILL.md" <<'EOF'
---
name: sua-van-dich
description: Sửa văn dịch tiếng Việt kém chất lượng. Dùng khi cần fix translated text, viết lại bản dịch, polish translation, sửa dịch máy.
argument-hint: "[đoạn văn hoặc file cần sửa]"
---

# Sửa Văn Dịch

> Tất cả tiếng Việt phải viết có dấu.

Skill chuyên biến văn dịch máy (word-by-word) thành tiếng Việt tự nhiên.

## Quy trình

1. **Đọc hiểu**: Đọc toàn bộ đoạn → hiểu plot, mood, nhân vật
2. **Phân loại lỗi**: Đánh dấu từng vấn đề (xem Bảng Lỗi)
3. **Xác định phong cách**: Thể loại → chọn giọng văn phù hợp
4. **Viết lại**: Từng đoạn, giữ nguyên nội dung, thay cách diễn đạt
5. **Kiểm tra**: Đọc lại liền mạch, check nhịp + xưng hô

## Bảng Lỗi Phổ Biến

### Cấu trúc câu (Structure)
| Lỗi | Ví dụ sai | Sửa |
|-----|-----------|-----|
| SVO cứng nhắc | "Anh ta đã đi đến cửa hàng để mua thức ăn" | "Anh ghé tiệm mua đồ ăn" |
| Quá nhiều "của" | "Ngôi nhà của cha của cô ấy" | "Nhà cha cô" |
| Câu dài không ngắt | Cả đoạn 1 câu | Tách thành 2-3 câu ngắn |
| Bị động thừa | "Cô ấy đã bị nhìn bởi anh ta" | "Anh nhìn cô" |

### Từ vựng (Vocabulary)
| Lỗi | Ví dụ sai | Sửa |
|-----|-----------|-----|
| Từ thừa | "cảm thấy rất vui vẻ và hạnh phúc" | "mừng rỡ" |
| Dịch literal idiom | "phá vỡ băng" | "mở lời" |
| Từ không tự nhiên | "thực hiện một nụ cười" | "mỉm cười" |
| Lặp đại từ | "Cô ấy...cô ấy...cô ấy" | "Cô...nàng...thiếu nữ" |

### Xưng hô (Pronouns)
| Thể loại | Nam chính | Nữ chính | Tôi (nam) | Tôi (nữ) |
|-----------|----------|----------|-----------|----------|
| Cổ trang/Tiên hiệp | hắn, y, chàng | nàng, ả, thiếu nữ | ta, bản tọa | ta, thiếp |
| Ngôn tình hiện đại | anh, anh ấy | cô, em | anh, tôi | em, tôi |
| Fantasy/Huyền huyễn | hắn, gã | nàng, cô gái | ta, bản vương | ta |
| Đô thị/Hiện đại | anh, hắn, gã | cô, cô gái | tôi | tôi |

### Hội thoại (Dialogue)
| Lỗi | Ví dụ sai | Sửa |
|-----|-----------|-----|
| Thiếu action beat | "Tôi yêu em," anh nói | "Tôi yêu em." Anh nắm chặt tay cô |
| Tags đơn điệu | nói...nói...nói | thì thầm, gầm gừ, cười nhạt |
| Quá formal | "Tôi xin phép được hỏi ngài" | "Cho hỏi..." (tùy context) |

## Phong Cách Theo Thể Loại

### Tiên hiệp / Tu tiên
- Giọng văn trang trọng, hùng tráng
- Nhiều Hán Việt: tu luyện, đột phá, cảnh giới, linh khí
- Xưng hô: ta/ngươi, bản tọa, tại hạ, đạo hữu

### Ngôn tình
- Giọng văn mềm mại, chi tiết tâm lý
- Ít Hán Việt, nhiều miêu tả cảm xúc
- Xưng hô: anh/em, mình/cậu

### Fantasy / Huyền huyễn
- Giọng văn epic, miêu tả chiến đấu mãnh liệt
- Thuật ngữ riêng: magic system, skill names giữ nguyên
- Xưng hô: tùy setting

### Đô thị / Hiện đại
- Giọng văn đời thường, nhanh
- Ít hoa mỹ, nhiều hội thoại
- Xưng hô: tôi/bạn, anh/em

Tham khảo chi tiết: [references/common-fixes.md](references/common-fixes.md)

$ARGUMENTS
EOF

cat > "$SKILL_DIR/sua-van-dich/references/common-fixes.md" <<'EOF'
# Bảng Sửa Nhanh

## Cụm từ dịch máy → tiếng Việt tự nhiên

| Dịch máy | Tiếng Việt |
|-----------|-----------|
| trong khi đó | lúc ấy |
| vào lúc này | bấy giờ |
| nói với anh ấy | bảo hắn |
| nhìn vào cô ấy | nhìn cô / ngắm nàng |
| cảm thấy rằng | thấy |
| đã từng | từng |
| bắt đầu + V | V luôn (bỏ "bắt đầu" khi thừa) |
| thực hiện | làm |
| vì lý do này | vì vậy / thế nên |
| vào thời điểm đó | lúc đó / khi ấy |
| một cách + adj | adv (nhanh chóng → nhanh) |
| có thể được nhìn thấy | thấy / trông thấy |
| mặc dù điều đó | dẫu vậy |
| không có gì ngoại trừ | chỉ có |
| đưa ra một quyết định | quyết định |
| thời gian trôi qua | thời gian dần trôi |

## Filler words cần bỏ/giảm
- "thật sự", "thực sự" (dùng ít, không câu nào cũng có)
- "một cách" + tính từ (thay bằng trạng từ)
- "đã" (chỉ dùng khi cần nhấn mạnh quá khứ)
- "được" (bỏ khi thừa: "được nhìn thấy" → "thấy")
- "rất" (thay bằng từ mạnh hơn: "rất đẹp" → "lộng lẫy")

## Nhịp văn
- Miêu tả cảnh: câu dài, nhiều tính từ, từ láy
- Hành động: câu ngắn, động từ mạnh, ít tính từ
- Cao trào: câu cực ngắn, ngắt dòng
- Tâm lý: câu trung bình, nhiều nội tâm
EOF

(cd "$SKILL_DIR" && zip -r sua-van-dich.zip sua-van-dich/)
log "Uploading skill: sua-van-dich..."
SKILL1_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/sua-van-dich.zip" 2>/dev/null)
SKILL1_CODE=$(echo "$SKILL1_RESP" | tail -1)
SKILL1_BODY=$(echo "$SKILL1_RESP" | sed '$d')
SKILL1_ID=$(echo "$SKILL1_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "sua-van-dich ID: ${SKILL1_ID:-failed ($SKILL1_CODE)}"

# ── Skill 2: viet-lai-hoi-thoai ──
log "Building skill: viet-lai-hoi-thoai..."
mkdir -p "$SKILL_DIR/viet-lai-hoi-thoai"

cat > "$SKILL_DIR/viet-lai-hoi-thoai/SKILL.md" <<'EOF'
---
name: viet-lai-hoi-thoai
description: Viết lại hội thoại trong truyện dịch. Dùng khi dialogue cứng nhắc, xưng hô sai, thiếu action beats, giọng nói không phân biệt.
argument-hint: "[đoạn hội thoại cần sửa] [thể loại] [mối quan hệ nhân vật]"
---

# Viết Lại Hội Thoại Truyện Dịch

## Vấn đề phổ biến trong hội thoại dịch máy

1. **Xưng hô sai**: Tất cả đều "anh ấy/cô ấy" — cần chuyển theo thể loại
2. **Quá formal**: "Tôi muốn thông báo rằng..." → "Này, ..."
3. **Thiếu cảm xúc**: Chỉ có lời thoại, không có hành động, biểu cảm
4. **Giọng nói giống nhau**: Mọi nhân vật nói cùng 1 style
5. **Info dump**: Giải thích exposition qua lời thoại gượng ép

## Quy trình

1. Xác định **mối quan hệ** giữa nhân vật → chọn xưng hô
2. Xác định **tính cách** mỗi nhân vật → chọn speech pattern
3. Viết lại với:
   - Action beats thay dialogue tags thừa
   - Subtext — nhân vật không nói thẳng mọi thứ
   - Nhịp: câu ngắn = căng thẳng, câu dài = suy tư
4. Check: bịt tên → đọc thoại → đoán được ai nói không?

## Speech Patterns Theo Tính Cách

| Tính cách | Cách nói |
|-----------|---------|
| Lạnh lùng | Câu ngắn, ít từ, không giải thích |
| Nóng nảy | Ngắt lời, câu cảm thán, thô ráp |
| Thông minh | Châm biếm, ẩn ý, câu phức |
| Hiền lành | Nhẹ nhàng, hay xin lỗi, dùng kính ngữ |
| Kiêu ngạo | Mệnh lệnh, hạ thấp người khác, xưng "bản tọa/ta" |

## Ví Dụ

**Dịch máy:**
> "Tôi nghĩ rằng chúng ta nên đi đến nơi đó," anh ta nói.
> "Tôi đồng ý với ý kiến của anh," cô ta nói.
> "Vậy thì chúng ta hãy đi," anh ta nói.

**Viết lại (tiên hiệp):**
> "Đi thôi." Hắn đứng dậy, phất tay áo.
> Nàng gật đầu, bước theo không chần chừ.
> Hai người hóa thành hai đạo lưu quang, xẹt về phía chân trời.

$ARGUMENTS
EOF

(cd "$SKILL_DIR" && zip -r viet-lai-hoi-thoai.zip viet-lai-hoi-thoai/)
log "Uploading skill: viet-lai-hoi-thoai..."
SKILL2_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/viet-lai-hoi-thoai.zip" 2>/dev/null)
SKILL2_CODE=$(echo "$SKILL2_RESP" | tail -1)
SKILL2_BODY=$(echo "$SKILL2_RESP" | sed '$d')
SKILL2_ID=$(echo "$SKILL2_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "viet-lai-hoi-thoai ID: ${SKILL2_ID:-failed ($SKILL2_CODE)}"

# ── Skill 3: kiem-tra-van-dich ──
log "Building skill: kiem-tra-van-dich..."
mkdir -p "$SKILL_DIR/kiem-tra-van-dich"

cat > "$SKILL_DIR/kiem-tra-van-dich/SKILL.md" <<'EOF'
---
name: kiem-tra-van-dich
description: Kiểm tra chất lượng bản viết lại từ truyện dịch. Dùng khi cần QA, review bản sửa, so sánh gốc vs viết lại.
argument-hint: "[file gốc] [file viết lại]"
---

# Kiểm Tra Văn Dịch

## Quy trình

1. Đọc bản gốc (dịch thô): `read_file("raw/{file}")`
2. Đọc bản viết lại: `read_file("fixed/{file}")`
3. So sánh từng section theo checklist
4. Sửa lỗi nhỏ trực tiếp, ghi note lỗi lớn
5. Ghi review report

## Checklist

### Nội dung (không được mất)
- [ ] Mọi sự kiện trong bản gốc đều có trong bản viết lại
- [ ] Không thêm sự kiện mới không có trong gốc
- [ ] Hội thoại giữ đúng ý nghĩa (dù cách nói khác)
- [ ] Tên nhân vật, địa danh chính xác
- [ ] Số liệu, thời gian, khoảng cách đúng

### Văn phong (phải tự nhiên)
- [ ] Không còn cấu trúc "anh ấy/cô ấy" lặp lại
- [ ] Không còn câu dịch word-by-word
- [ ] Xưng hô nhất quán theo thể loại
- [ ] Nhịp văn đa dạng (ngắn-dài xen kẽ)
- [ ] Hội thoại phân biệt giọng nhân vật
- [ ] Từ láy, thành ngữ dùng đúng chỗ

### Red flags (cần sửa ngay)
- [ ] Tên nhân vật thay đổi giữa chừng
- [ ] Xưng hô nhảy loạn (lúc "hắn" lúc "anh ấy")
- [ ] Scene bị bỏ sót hoàn toàn
- [ ] Ý nghĩa bị đảo ngược (phủ định → khẳng định)

## Scoring

| Score | Meaning |
|-------|---------|
| 9-10 | Đọc như truyện gốc tiếng Việt |
| 7-8 | Tốt, vài chỗ hơi cứng |
| 5-6 | Trung bình, còn nhiều câu dịch máy |
| <5 | Cần viết lại |

## Output

```markdown
# Review: {filename}
## Score: X/10
## Nội dung: OK / Thiếu scene X
## Lỗi văn phong
- Line X: còn cứng "..." → nên "..."
## Xưng hô
- Nhân vật A: OK / Không nhất quán (lúc X lúc Y)
## Kết luận
- PASS / CẦN SỬA / CẦN VIẾT LẠI
```

$ARGUMENTS
EOF

(cd "$SKILL_DIR" && zip -r kiem-tra-van-dich.zip kiem-tra-van-dich/)
log "Uploading skill: kiem-tra-van-dich..."
SKILL3_RESP=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/v1/skills/upload" \
  -H "Authorization: Bearer $TOKEN" -H "X-GoClaw-User-Id: $USER_ID" \
  -F "file=@${SKILL_DIR}/kiem-tra-van-dich.zip" 2>/dev/null)
SKILL3_CODE=$(echo "$SKILL3_RESP" | tail -1)
SKILL3_BODY=$(echo "$SKILL3_RESP" | sed '$d')
SKILL3_ID=$(echo "$SKILL3_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
info "kiem-tra-van-dich ID: ${SKILL3_ID:-failed ($SKILL3_CODE)}"

fi  # end SKIP_SKILLS

# ═══════════════════════════════════════════════
#  STEP 4: Grant skills to agents
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
  api POST "/v1/skills/${skill_id}/grants/agent" -d "{\"agent_id\":\"${agent_id}\"}" > /dev/null 2>&1 || true
}

SKILL_IDS=("${SKILL1_ID:-}" "${SKILL2_ID:-}" "${SKILL3_ID:-}")
SKILL_NAMES=("sua-van-dich" "viet-lai-hoi-thoai" "kiem-tra-van-dich")
AGENT_IDS=("${LEAD_ID:-}" "${REWRITER_ID:-}" "${REVIEWER_ID:-}")
AGENT_NAMES=("lead" "rewriter" "reviewer")

for i in 0 1 2; do
  for j in 0 1 2; do
    grant_skill "${SKILL_IDS[$i]}" "${AGENT_IDS[$j]}" "${SKILL_NAMES[$i]}" "${AGENT_NAMES[$j]}"
  done
done

# ═══════════════════════════════════════════════
#  STEP 5: Create Team
# ═══════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Step 5: Creating Novel Fixer Team"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

log "Creating team via REST API..."
TEAM_RESP=$(api POST /v1/teams -d "$(cat <<'JSON'
{
  "name": "Novel Fixer Team",
  "lead": "bien-tap-dich",
  "members": ["viet-lai", "kiem-duyet-dich"],
  "description": "Sửa truyện dịch tiếng Việt kém chất lượng: Lead phân task → Viết Lại sửa văn → Kiểm Duyệt review",
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
JSON
)" || true)

if [[ -n "$TEAM_RESP" ]]; then
  TEAM_ID=$(echo "$TEAM_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  info "Team ID: ${TEAM_ID:-check response}"
else
  warn "Failed to create team"
  TEAM_ID=""
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
echo "    bien-tap-dich       ${LEAD_ID:-N/A}"
echo "    viet-lai            ${REWRITER_ID:-N/A}"
echo "    kiem-duyet-dich     ${REVIEWER_ID:-N/A}"
echo ""
echo "  Skills:"
echo "    sua-van-dich        ${SKILL1_ID:-N/A}"
echo "    viet-lai-hoi-thoai  ${SKILL2_ID:-N/A}"
echo "    kiem-tra-van-dich   ${SKILL3_ID:-N/A}"
echo ""
echo "  Team: Novel Fixer     ${TEAM_ID:-N/A}"
echo ""
echo "  Cách dùng:"
echo "    1. Mở Web UI → chat với 'Biên Tập Dịch'"
echo "    2. Paste đoạn truyện dịch thô, nói: 'Sửa lại đoạn này cho mượt'"
echo "    3. Pipeline: Lead → Viết Lại → Kiểm Duyệt → Bản hoàn chỉnh"
echo ""
