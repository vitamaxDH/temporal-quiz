# Repo Split: Worker (private) + UI (public) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split `temporal-quiz` into two repos: `temporal-quiz` (private, all Go/worker code) and `temporal-quiz-ui` (public, static web + quiz JSONs).

**Architecture:** The worker writes quiz JSON files to a configurable output directory via `TEMPORAL_QUIZ_OUTPUT_DIR`. On the dev machine this points to the UI repo's `web/quizzes/`. The UI repo is a standalone static site deployed via GitHub Pages.

**Tech Stack:** Go 1.26, Temporal SDK, vanilla HTML/CSS/JS, GitHub Pages

---

## Repo Layout After Split

```
~/project/temporal/
├── temporal-quiz/          ← PRIVATE (existing repo, keep Go code)
│   ├── cmd/
│   ├── quiz/
│   ├── scraper/
│   ├── config/
│   ├── temporal-docker-compose/
│   ├── go.mod, go.sum
│   └── Makefile
│
└── temporal-quiz-ui/       ← PUBLIC (new repo, static site)
    ├── web/
    │   ├── index.html
    │   ├── app.js
    │   ├── style.css
    │   ├── *.svg, *.png, favicon.ico
    │   └── quizzes/        ← JSON output from worker
    ├── .github/workflows/deploy.yml
    └── Makefile
```

---

## Task 1: Add `TEMPORAL_QUIZ_OUTPUT_DIR` support to worker

**Files:**
- Modify: `cmd/worker/main.go`
- Modify: `cmd/localgen/main.go`

**Step 1: Update `cmd/worker/main.go` to read output dir from env**

Change the hardcoded `quiz.DefaultOutputDir` to read from env:

```go
// Replace:
quizActivities := &quiz.QuizActivities{
    ...
    OutputDir:  quiz.DefaultOutputDir,
    ...
}

// With:
outputDir := os.Getenv("TEMPORAL_QUIZ_OUTPUT_DIR")
if outputDir == "" {
    outputDir = quiz.DefaultOutputDir
}
quizActivities := &quiz.QuizActivities{
    ...
    OutputDir:  outputDir,
    ...
}
```

Also add `"os"` to imports if not already present.

**Step 2: Check `cmd/localgen/main.go` for OutputDir usage**

Open the file, find where `quiz.DefaultOutputDir` or `quiz.QuizActivities{OutputDir: ...}` is set, apply the same env var pattern:

```go
outputDir := os.Getenv("TEMPORAL_QUIZ_OUTPUT_DIR")
if outputDir == "" {
    outputDir = quiz.DefaultOutputDir
}
```

**Step 3: Build to verify it compiles**

```bash
cd ~/project/temporal/temporal-quiz
go build ./...
```
Expected: no errors.

**Step 4: Commit**

```bash
git add cmd/worker/main.go cmd/localgen/main.go
git commit -m "feat: support TEMPORAL_QUIZ_OUTPUT_DIR env var for configurable quiz output path"
```

---

## Task 2: Create the `temporal-quiz-ui` repo

**Files:**
- Create: `~/project/temporal/temporal-quiz-ui/`

**Step 1: Create directory and copy web assets**

```bash
mkdir -p ~/project/temporal/temporal-quiz-ui
cp -r ~/project/temporal/temporal-quiz/web ~/project/temporal/temporal-quiz-ui/web
mkdir -p ~/project/temporal/temporal-quiz-ui/.github/workflows
cp ~/project/temporal/temporal-quiz/.github/workflows/deploy.yml \
   ~/project/temporal/temporal-quiz-ui/.github/workflows/deploy.yml
```

**Step 2: Create `Makefile` in UI repo**

Create `~/project/temporal/temporal-quiz-ui/Makefile`:

```makefile
.PHONY: serve

serve:
	@echo "Serving quiz UI at http://localhost:8080..."
	@cd web && python3 -m http.server 8080
```

**Step 3: Create `.gitignore` in UI repo**

Create `~/project/temporal/temporal-quiz-ui/.gitignore`:

```
.DS_Store
```

Note: quiz JSON files ARE committed in the UI repo — they are the public content.

**Step 4: Init git and make initial commit**

```bash
cd ~/project/temporal/temporal-quiz-ui
git init
git add .
git commit -m "feat: initial commit — temporal-quiz UI (public)"
```

Expected: clean commit with web/ assets and GitHub Actions workflow.

---

## Task 3: Remove `web/` and `.github/` from the worker repo

**Files:**
- Delete: `web/` directory
- Delete: `.github/` directory
- Modify: `Makefile`
- Modify: `.gitignore`

**Step 1: Remove web and github dirs**

```bash
cd ~/project/temporal/temporal-quiz
rm -rf web/ .github/
```

**Step 2: Update `Makefile` — remove web/pages targets**

Remove these targets entirely:
- `serve-web`
- `wipe-all` (references `web/quizzes/*.json`) — or update it to only clean `temporal_docs_html/` and `bin/`

Updated `Makefile` targets list should be:
```
build test lint worker scrape quizgen localgen pipeline start-server stop-server clean
```

Also update the `help` target and `.PHONY` line to remove the deleted targets.

Update `wipe-all`:
```makefile
wipe-all: clean
	@echo "Wiping all processed txt files and binaries..."
	@rm -rf temporal_docs_txt bin/
```

**Step 3: Update `.gitignore` — remove web-related entries if any**

Check `.gitignore` for any `web/quizzes` references and remove them.

**Step 4: Verify build still works**

```bash
cd ~/project/temporal/temporal-quiz
go build ./...
```
Expected: no errors (no Go code references `web/` directly).

**Step 5: Commit**

```bash
git add -A
git commit -m "chore: remove web UI and GitHub Pages — moved to temporal-quiz-ui repo"
```

---

## Task 4: Add `TEMPORAL_QUIZ_OUTPUT_DIR` to `~/.zshrc`

**Step 1: Append to `~/.zshrc`**

Add below the existing `TEMPORAL_QUIZ_*` exports:

```bash
export TEMPORAL_QUIZ_OUTPUT_DIR=/Users/daehan/project/temporal/temporal-quiz-ui/web/quizzes
```

**Step 2: Reload and verify**

```bash
source ~/.zshrc
echo $TEMPORAL_QUIZ_OUTPUT_DIR
```
Expected: `/Users/daehan/project/temporal/temporal-quiz-ui/web/quizzes`

**Step 3: Test end-to-end (optional smoke test)**

If the Temporal worker is running, trigger `make quizgen` and confirm JSON files appear in `~/project/temporal/temporal-quiz-ui/web/quizzes/`.

---

## Task 5: Set up remote for `temporal-quiz-ui`

**Step 1: Create public GitHub repo**

Go to GitHub and create `temporal-quiz-ui` as a **public** repo (no README, no .gitignore — we already have them).

**Step 2: Push**

```bash
cd ~/project/temporal/temporal-quiz-ui
git remote add origin git@github.com:<your-username>/temporal-quiz-ui.git
git push -u origin main
```

**Step 3: Enable GitHub Pages**

In the repo settings → Pages → Source: GitHub Actions. The existing `deploy.yml` will handle deploys on push to `main` when `web/**` changes.

---

## Post-Split Workflow

When you run `make pipeline` or `make quizgen` in the worker repo:
1. Worker generates quiz JSONs → writes to `$TEMPORAL_QUIZ_OUTPUT_DIR` (= `temporal-quiz-ui/web/quizzes/`)
2. `cd ~/project/temporal/temporal-quiz-ui && git add web/quizzes/ && git commit -m "chore: update quiz content" && git push`
3. GitHub Actions deploys to Pages automatically.
