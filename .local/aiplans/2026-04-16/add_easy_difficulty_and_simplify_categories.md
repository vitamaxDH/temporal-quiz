# Add 4 Difficulty Tiers, Simplify Categories, Add Calculator

**Goal:** Expand difficulty from 2 tiers (hard/nightmare) to 4 (easy/med/hard/nightmare), merge all language-specific Develop_* categories into "Develop", and add an in-quiz calculator widget.

**Success criteria:**
- 4 difficulty tiers generate with distinct prompts
- Difficulty dots scale 1-4 (easy=1, med=2, hard=3, nightmare=4)
- All `Develop_*` categories merge into single `Develop`
- Calculator is accessible during quiz without leaving the question
- All tests pass

---

## Phase 1: Add Easy + Med Difficulty Tiers

### 1.1 New prompts in `quiz/prompt.go`

**`EasyPrompt()`** - beginner-friendly, concept-level, no code:
```go
func EasyPrompt(n int, bucketText string) string {
    return fmt.Sprintf(`You are a Temporal platform expert creating beginner-friendly quiz questions. Your goal is to help newcomers build foundational understanding of Temporal concepts.

RULES:
- Questions should test UNDERSTANDING of core concepts, not memorization
- Use simple, clear language with minimal jargon
- Wrong answers should be plausible but clearly distinguishable with basic knowledge
- The explanation should teach the concept from scratch, assuming the reader is new to Temporal
- Focus on "what" and "why" rather than edge cases or production gotchas
- Do NOT include code snippets or require SDK-specific knowledge

Generate %d easy multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"B","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}
```

**`MedPrompt()`** - practical understanding, may include simple code, but not tricky:
```go
func MedPrompt(n int, bucketText string) string {
    return fmt.Sprintf(`You are a Temporal platform expert creating intermediate quiz questions. Your goal is to help engineers solidify their practical understanding of Temporal patterns and APIs.

RULES:
- Questions test PRACTICAL KNOWLEDGE of how Temporal features work
- May include simple code snippets (Go or Python) showing common patterns
- Wrong answers should represent reasonable misunderstandings, not absurd options
- The explanation should clarify the concept and mention when you would use this pattern in practice
- Focus on "how it works" and correct usage, not extreme edge cases
- Questions should be answerable by someone who has built a few Temporal workflows

Generate %d medium-difficulty multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"B","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}
```

### 1.2 Update types in `quiz/types.go`

**ManifestEntry:**
```go
type ManifestEntry struct {
    Category       string `json:"category"`
    QuestionCount  int    `json:"question_count"`
    EasyCount      int    `json:"easy_count"`
    MedCount       int    `json:"med_count"`
    HardCount      int    `json:"hard_count"`
    NightmareCount int    `json:"nightmare_count"`
}
```

**QuizGenParams:**
```go
type QuizGenParams struct {
    EasyCount      int      // easy questions per category (default 3, max 30)
    MedCount       int      // med questions per category (default 4, max 30)
    HardCount      int      // hard questions per category (default 4, max 30)
    NightmareCount int      // nightmare questions per category (default 2, max 30)
    Categories     []string // filter to specific categories (empty = all)
}
```

Default split: 3 easy + 4 med + 4 hard + 2 nightmare = 13 per category.

**GenerateQuizInput:**
```go
type GenerateQuizInput struct {
    BucketPath     string
    Category       string
    EasyCount      int
    MedCount       int
    HardCount      int
    NightmareCount int
}
```

### 1.3 Update `GenerateQuiz` in `quiz/activities.go`

Add easy and med Claude calls alongside hard and nightmare:

```go
easyRaw, err := a.callClaude(ctx, EasyPrompt(input.EasyCount, bucketText))
// ...
medRaw, err := a.callClaude(ctx, MedPrompt(input.MedCount, bucketText))
// ...
hardRaw, err := a.callClaude(ctx, HardPrompt(input.HardCount, bucketText))
// ...
nightmareRaw, err := a.callClaude(ctx, NightmarePrompt(input.NightmareCount, bucketText))
```

Build questions in order: easy -> med -> hard -> nightmare.

### 1.4 Update `ListBuckets` in `quiz/activities.go`

Set all four counts to `DefaultQuestionsPerCat` in the generated inputs.

### 1.5 Update `WriteQuizFiles` in `quiz/activities.go`

Track all four difficulty counts in the switch:
```go
var easyCount, medCount, hardCount, nightmareCount int
for _, q := range qs {
    switch q.Difficulty {
    case "easy":
        easyCount++
    case "med":
        medCount++
    case "hard":
        hardCount++
    case "nightmare":
        nightmareCount++
    }
}
```

### 1.6 Update `QuizGeneratorWorkflow` in `quiz/workflow.go`

Add defaults and caps for all four:
```go
if params.EasyCount <= 0 { params.EasyCount = 3 }
if params.MedCount <= 0 { params.MedCount = 4 }
if params.HardCount <= 0 { params.HardCount = 4 }
if params.NightmareCount <= 0 { params.NightmareCount = 2 }
// caps at 30 for each
```

Wire all four into bucket inputs in the fan-out loop.

### 1.7 Update CLI in `cmd/quizgen/main.go`

```go
easy := flag.Int("easy", 3, "number of easy questions per category (max 30)")
med := flag.Int("med", 4, "number of med questions per category (max 30)")
hard := flag.Int("hard", 4, "number of hard questions per category (max 30)")
nightmare := flag.Int("nightmare", 2, "number of nightmare questions per category (max 30)")
```

### 1.8 Update `cmd/pipeline/main.go`

```go
params := quiz.QuizGenParams{
    EasyCount:      3,
    MedCount:       4,
    HardCount:      4,
    NightmareCount: 2,
}
```

### 1.9 Update frontend `difficultyDots()` in `web/app.js`

Change from 3-dot to 4-dot scale:
```javascript
function difficultyDots(difficulty) {
  const levels = { easy: 1, med: 2, hard: 3, nightmare: 4 };
  const filled = levels[difficulty] || 1;
  const total = 4;
  let html = '';
  for (let i = 0; i < total; i++) {
    html += `<span class="dot ${i < filled ? 'filled' : 'empty'}"></span>`;
  }
  return html + ' ' + difficulty;
}
```

---

## Phase 2: Merge Language Categories into Single "Develop"

### 2.1 Simplify `scraper/buckets.go`

**Remove language-specific switch block** (lines 45-54).

**Merge Buckets map:**
- Remove `Develop_Other_SDKs` and `Develop_General`
- Add single `"Develop": {"develop"}`

Since `"develop"` prefix matches all `develop_go_*`, `develop_java_*`, etc., one entry catches everything.

**Remove `Develop_General` special-case logic** (lines 58-64).

**Update `bucketOrder`:** Replace `Develop_Other_SDKs` and `Develop_General` with `Develop`.

**Update `SortedBucketKeys()`:** Replace all `Develop_*` entries with single `"Develop"`.

### 2.2 Update `scraper/buckets_test.go`

All language-specific test cases expect `"Develop"`:
```go
{"develop_go_workers.html", "Develop"},
{"develop_java_activities.html", "Develop"},
{"develop_python_workflows.html", "Develop"},
{"develop_typescript_signals.html", "Develop"},
{"develop_dotnet_overview.html", "Develop"},
{"develop_php_activities.html", "Develop"},
{"develop_ruby_workers.html", "Develop"},
{"develop_index.html", "Develop"},
```

### 2.3 Improve `formatCategoryLabel()` in `web/app.js`

```javascript
function formatCategoryLabel(category) {
  return category.replace(/_/g, ' ').replace(/ and /g, ' & ');
}
```

Results:
- `Features_Workers_and_Routing` -> `Features Workers & Routing`
- `CLI_and_References` -> `CLI & References`
- `Self_Hosted_and_Ops` -> `Self Hosted & Ops`
- `Develop` -> `Develop`

---

## Phase 3: Add Calculator Widget

### 3.1 HTML in `web/index.html`

Add a calculator toggle button and panel near the actions area:

```html
<div class="calculator-container">
  <button class="btn btn-ghost btn-calc-toggle" onclick="toggleCalculator()">Calc</button>
  <div class="calculator" id="calculator" style="display:none;">
    <input type="text" class="calc-display" id="calcDisplay" readonly>
    <div class="calc-buttons">
      <button onclick="calcInput('7')">7</button>
      <button onclick="calcInput('8')">8</button>
      <button onclick="calcInput('9')">9</button>
      <button onclick="calcInput('/')">/</button>
      <button onclick="calcInput('4')">4</button>
      <button onclick="calcInput('5')">5</button>
      <button onclick="calcInput('6')">6</button>
      <button onclick="calcInput('*')">*</button>
      <button onclick="calcInput('1')">1</button>
      <button onclick="calcInput('2')">2</button>
      <button onclick="calcInput('3')">3</button>
      <button onclick="calcInput('-')">-</button>
      <button onclick="calcInput('0')">0</button>
      <button onclick="calcInput('.')">.</button>
      <button onclick="calcEval()">=</button>
      <button onclick="calcInput('+')">+</button>
      <button onclick="calcClear()" class="calc-clear">C</button>
    </div>
  </div>
</div>
```

### 3.2 CSS in `web/style.css`

```css
/* Calculator */
.calculator-container {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 50;
}
.btn-calc-toggle {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
}
.calculator {
  position: absolute;
  bottom: 56px;
  right: 0;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 12px;
  width: 220px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.4);
}
.calc-display {
  width: 100%;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 16px;
  padding: 8px 10px;
  text-align: right;
  margin-bottom: 8px;
}
.calc-buttons {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
}
.calc-buttons button {
  font-family: var(--font-mono);
  font-size: 13px;
  padding: 10px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.1s;
}
.calc-buttons button:hover {
  background: var(--bg-hover);
}
.calc-clear {
  grid-column: span 4;
  background: var(--bg-hover) !important;
  color: var(--wrong) !important;
}
```

### 3.3 JavaScript in `web/app.js`

```javascript
/* ---- Calculator ---- */

function toggleCalculator() {
  const calc = document.getElementById('calculator');
  calc.style.display = calc.style.display === 'none' ? 'block' : 'none';
}

function calcInput(val) {
  const display = document.getElementById('calcDisplay');
  display.value += val;
}

function calcClear() {
  document.getElementById('calcDisplay').value = '';
}

function calcEval() {
  const display = document.getElementById('calcDisplay');
  try {
    // Only allow numbers, operators, dots, and parens
    const sanitized = display.value.replace(/[^0-9+\-*/.() ]/g, '');
    display.value = Function('"use strict"; return (' + sanitized + ')')();
  } catch {
    display.value = 'Error';
  }
}
```

---

## Phase 4: Update Tests

### 4.1 `quiz/prompt_test.go`

Add tests for `EasyPrompt` and `MedPrompt`:
```go
func TestEasyPrompt(t *testing.T) {
    result := EasyPrompt(3, "beginner docs")
    assert.Contains(t, result, "3 easy multiple-choice")
    assert.Contains(t, result, "beginner docs")
    assert.Contains(t, result, "beginner-friendly")
}

func TestMedPrompt(t *testing.T) {
    result := MedPrompt(4, "intermediate docs")
    assert.Contains(t, result, "4 medium-difficulty")
    assert.Contains(t, result, "intermediate docs")
    assert.Contains(t, result, "PRACTICAL KNOWLEDGE")
}
```

### 4.2 `quiz/activities_test.go`

- `TestListBuckets_Success`: Assert `EasyCount` and `MedCount` fields
- `TestGenerateQuiz_Success`: Expect 4 questions (1 easy + 1 med + 1 hard + 1 nightmare). Verify each difficulty.
- `TestWriteQuizFiles_Success`: Add easy/med questions to input, verify all counts in manifest.

### 4.3 `quiz/workflow_test.go`

- All `QuizGenParams` include `EasyCount` and `MedCount`

### 4.4 `scraper/buckets_test.go`

- All `Develop_*` expectations changed to `"Develop"`

---

## Phase 5: Verify

- `go build ./...`
- `go test ./...`
- Manual check: serve frontend, verify 4-dot difficulty scale, calculator works, "Develop" is one pill

---

## Summary of files changed

| File | Changes |
|------|---------|
| `quiz/prompt.go` | Add `EasyPrompt()`, `MedPrompt()` |
| `quiz/types.go` | Add `EasyCount`, `MedCount` to ManifestEntry, QuizGenParams, GenerateQuizInput |
| `quiz/activities.go` | Call Easy/Med prompts, track all 4 difficulties, set counts in ListBuckets |
| `quiz/workflow.go` | Add Easy/Med defaults/caps, wire to buckets |
| `cmd/quizgen/main.go` | Add `-easy`, `-med` flags, update defaults |
| `cmd/pipeline/main.go` | Add `EasyCount: 3, MedCount: 4`, adjust hard/nightmare defaults |
| `scraper/buckets.go` | Merge all Develop_* into "Develop", remove language switch |
| `scraper/buckets_test.go` | Update expected categories to "Develop" |
| `quiz/prompt_test.go` | Add TestEasyPrompt, TestMedPrompt |
| `quiz/activities_test.go` | Update for Easy/Med fields, add assertions |
| `quiz/workflow_test.go` | Include Easy/Med counts in test params |
| `web/app.js` | Update difficultyDots to 4-dot, improve formatCategoryLabel, add calculator JS |
| `web/index.html` | Add calculator HTML |
| `web/style.css` | Add calculator CSS |
