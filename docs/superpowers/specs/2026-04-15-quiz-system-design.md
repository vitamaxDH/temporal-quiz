# Temporal Quiz System Design

## Summary

A daily Temporal workflow that generates educational, scenario-based multiple-choice quiz questions from scraped Temporal docs using OpenAI. Questions teach real production concepts and help Temporal engineers grow. Served as a static site on GitHub Pages with localStorage-based progress tracking.

## Architecture

**Monorepo** (`temporal-quiz`). Three concerns:
1. **Scraper** (existing): crawls docs.temporal.io, outputs bucketed Markdown text files
2. **Quiz Generator** (new): reads text buckets, calls OpenAI, outputs quiz JSON
3. **Web UI** (new): static HTML/CSS/JS site that loads quiz JSON, tracks progress in localStorage

```
Daily Pipeline:
  ScraperWorkflow -> QuizGeneratorWorkflow -> JSON files committed to web/quizzes/
  
GitHub Pages serves web/ directory
```

## Quiz Data Model

Each question:
```json
{
  "id": "features_workflows_001",
  "category": "Features_Workflows",
  "difficulty": "hard|nightmare",
  "question": "scenario text, may include `code`",
  "choices": [
    {"key": "A", "text": "answer text"},
    {"key": "B", "text": "answer text"},
    {"key": "C", "text": "answer text"},
    {"key": "D", "text": "answer text"}
  ],
  "answer": "B",
  "explanation": "Why correct + why the tempting wrong answer is wrong",
  "source_doc": "workflow-timeouts.html",
  "generated_at": "2026-04-15"
}
```

Per-category JSON files: `web/quizzes/{category}.json`
Manifest: `web/quizzes/manifest.json` with category names and question counts.

## Difficulty Tiers

The goal is **learning, not tricking**. Difficulty comes from the complexity of real-world scenarios, not from deceptive wording. Wrong answers represent common misconceptions worth correcting. The explanation is the most valuable part of every question: it's where the actual learning happens.

**Hard** (2 dots): Real-world production scenario that teaches an important Temporal concept. Wrong answers represent common misconceptions that engineers actually make. The explanation teaches WHY the correct behavior works that way and what would go wrong if you assumed otherwise.

**Nightmare** (3 dots): Complex multi-feature production scenario where understanding how 2-3 Temporal features interact is essential. May include code snippets. The explanation goes deep into the "why" and connects to broader Temporal design principles. After reading the explanation, the engineer should understand a concept they can apply beyond this specific question.

Mix per category: ~7 hard + ~3 nightmare per batch of 10 questions.

## Workflow Design

### DailyPipelineWorkflow
Orchestrates the full daily pipeline.

1. Execute `ScraperWorkflow` as a child workflow (existing)
2. Execute `QuizGeneratorWorkflow` as a child workflow (new)
3. Return summary stats

Scheduled via Temporal Schedule (cron: daily).

### QuizGeneratorWorkflow
Generates quiz questions for all categories.

1. `ListBucketsActivity`: scan `temporal_docs_txt/` directory, return list of bucket file paths
2. Fan-out: `GenerateQuizActivity` per bucket (parallel, up to 20 concurrent)
   - Read the bucket text file
   - Call OpenAI API (gpt-4o) with the scenario-based prompt
   - Parse the response into `[]QuizQuestion`
   - Return questions for this category
3. Fan-in: collect all questions
4. `WriteQuizFilesActivity`: write per-category JSON + manifest to `web/quizzes/`
5. Return total question count

**Retry policy**: GenerateQuizActivity gets 3 retries with 5s initial interval (handles OpenAI rate limits). Write activity gets 2 retries.

## OpenAI Prompt Design

Two prompt variants, one per difficulty. Both prioritize teaching over tricking.

### Hard prompt
```
You are a Temporal platform expert creating educational quiz questions that help 
engineers deeply understand Temporal. Your goal is to help people LEARN, not to 
trick them. Every question should teach something valuable about how Temporal 
works in production.

RULES:
- Every question describes a REAL-WORLD PRODUCTION SCENARIO
- Wrong answers represent COMMON MISCONCEPTIONS that engineers actually make
- The explanation is the most important part: explain WHY the correct answer is 
  right, what the underlying Temporal design principle is, and what would go 
  wrong if you assumed otherwise
- After answering, the reader should understand a concept they can apply to 
  their own Temporal code
- Do NOT write definition/recall questions like "What is X?"
- Do NOT make questions tricky for the sake of being tricky

Generate {N} hard multiple-choice questions from this documentation:
{bucket_text}

Return a JSON array matching this exact schema:
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"B","explanation":"...","source_doc":"filename.html"}]
```

### Nightmare prompt
```
You are a Temporal platform expert creating advanced educational quiz questions 
for experienced engineers. These questions teach how multiple Temporal features 
interact in complex production scenarios. The goal is growth, not gotchas.

RULES:
- Questions describe COMPLEX PRODUCTION SCENARIOS where 2-3 Temporal features 
  interact (e.g., child workflows + signals + timeouts, versioning + continue-as-new)
- May include Go or Python code snippets showing real workflow/activity patterns
- Wrong answers represent things an engineer might reasonably believe before 
  understanding the deeper behavior
- The explanation should go deep: explain the underlying design principle, connect 
  it to broader Temporal architecture, and give the reader an insight they can 
  apply beyond this specific question
- After reading the explanation, the engineer should think "I'm glad I learned 
  that before hitting it in production"

Generate {N} nightmare-difficulty multiple-choice questions from this documentation:
{bucket_text}

Return a JSON array matching the same schema as above.
```

Model: `gpt-4o` (best at complex reasoning for question generation).

## Web UI

### Tech
- Static HTML/CSS/JS (no framework, no build step)
- Hosted on GitHub Pages from `web/` directory
- Loads quiz JSON via `fetch()` from relative paths

### Design: "Terminal Elegance"
- Fonts: IBM Plex Mono (labels, code) + Outfit (body text)
- Dark theme with violet accent (#8b5cf6)
- Grain texture overlay for depth
- Answer reveal animation: correct=green, wrong=red, others dim
- Progress ring showing question N/total

### Pages
- **Home/Quiz**: category pills at top, quiz card centered, answer list below
- **Stats** (stretch): accuracy per category, streak counter, total questions answered

### State (localStorage)
```json
{
  "answers": {
    "features_workflows_001": {"selected": "B", "correct": true, "timestamp": 1234567890}
  },
  "stats": {
    "Features_Workflows": {"correct": 7, "total": 10},
    "Develop_Go": {"correct": 3, "total": 5}
  },
  "streak": 3
}
```

### Quiz Flow
1. User picks a category (or "All")
2. Load questions from JSON, shuffle, filter out already-answered (optional)
3. Show one question at a time
4. User selects answer, clicks Submit
5. Reveal: highlight correct/wrong, show explanation, animate
6. Save result to localStorage, update stats
7. "Next" loads the next question

### Weak Area Prioritization
When "All" category is selected, weight question selection toward categories with lower accuracy. Simple formula: categories below 70% accuracy get 2x weight in random selection.

## File Structure (new files)

```
temporal-quiz/
├── quiz/                        # New package
│   ├── workflow.go              # QuizGeneratorWorkflow + DailyPipelineWorkflow
│   ├── workflow_test.go
│   ├── activities.go            # ListBuckets, GenerateQuiz, WriteQuizFiles
│   ├── activities_test.go
│   ├── prompt.go                # Prompt templates
│   └── types.go                 # QuizQuestion, Manifest structs
├── cmd/
│   ├── worker/main.go           # Update: register quiz workflows/activities
│   ├── starter/main.go          # Update: option to run quiz generator
│   └── pipeline/main.go         # New: triggers DailyPipelineWorkflow
├── web/                         # Static site
│   ├── index.html               # Quiz UI
│   ├── style.css                # Styles
│   ├── app.js                   # Quiz logic
│   └── quizzes/                 # Generated JSON (gitignored until first run)
│       ├── manifest.json
│       ├── Features_Workflows.json
│       └── ...
├── config/config.go             # Update: add OPENAI_API_KEY env var
└── Makefile                     # Update: add generate-quizzes, pipeline targets
```

## Configuration (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `OPENAI_API_KEY` | (required for quiz gen) | OpenAI API key |
| `OPENAI_MODEL` | `gpt-4o` | Model for question generation |
| `QUIZ_OUTPUT_DIR` | `web/quizzes` | Where to write quiz JSON |
| `QUESTIONS_PER_CATEGORY` | `10` | Questions generated per bucket |

## Testing Strategy

- **Unit tests**: prompt template rendering, JSON parsing of OpenAI response, bucket listing, question dedup
- **Workflow tests**: mock OpenAI activity, verify fan-out/fan-in, verify JSON output structure
- **Activity tests**: mock HTTP server for OpenAI responses, verify prompt construction, verify file writes
- **Web UI**: manual testing via gstack browse (functional), visual inspection
