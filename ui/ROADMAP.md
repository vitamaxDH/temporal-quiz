# Roadmap

Ideas for upgrading the quiz app, grouped by theme and ranked by impact.

---

## Study mechanics (highest retention value)

| # | Idea | Why | Cost |
|---|------|-----|------|
| 1 | **Spaced repetition for wrong answers** | Wrong questions re-queue on an expanding schedule (1d / 3d / 7d). Turns the quiz from trivia into a real study loop. | Medium |
| 2 | **"Review mistakes" mode** | One-click session of recent wrong answers. Data already captured in `sessionWrongItems` + per-session `history`. | Small |
| 3 | **Bookmark / flag questions** | Star hard ones during a quiz, filter to them later. | Small |

## Analytics (data we already track)

| # | Idea | Why | Cost |
|---|------|-----|------|
| 4 | **Per-category accuracy chart** | Horizontal bars on landing / history showing where you're weak. `state.categoryStats` is already populated. | Small |
| 5 | **Streak calendar heatmap** | GitHub-style 7×5 grid of daily activity. Builds a "don't break the chain" habit. | Small / Medium |
| 6 | **Accuracy over time line** | Simple SVG line chart across recent sessions. | Small |

## Content depth

| # | Idea | Why | Cost |
|---|------|-----|------|
| 7 | **Link to source doc per question** | `source_doc` field already exists on every question. One click → open the Temporal docs page the Q came from. | Tiny |
| 8 | **Source snippet in explanation** | Worker already has the doc text — emit the paragraph the Q was derived from. Worker + UI change. | Medium |

## Distribution / UX

| # | Idea | Why | Cost |
|---|------|-----|------|
| 9 | **Mobile PWA** (offline + installable) | Home-screen icon, offline cache, optional daily reminder notifications. | Medium |
| 10 | **Search across questions + notes** | Once sessions pile up, finding "that one question about Nexus" matters. | Small |
| 11 | **Keyboard shortcut overlay (`?` key)** | Currently `N`, `Enter`, `A-D` exist but are undocumented. One modal fixes discoverability. | Tiny |
| 12 | **Shareable daily challenge link** | `example.com/?daily=YYYY-MM-DD` with seeded question order, comparable scores. Growth hook. | Medium |
| 13 | **Voice mode** | Web Speech API reads the question aloud. Good for walk / commute study. | Small |

---

## Recommended ship order

If I had to pick three that compound each other:

1. **#2 Review mistakes mode** — biggest single win for the "spot your gaps" promise in the About page. ~100 lines.
2. **#4 Per-category accuracy chart + #5 Streak calendar** — stack both into a "stats" tab in the History drawer. One screen, two visualizations, zero new navigation.
3. **#7 Source doc link per question** — cheapest, most learning-boosting single line. Every question in localStorage already has `source_doc`.

## Holding off on

- **Leaderboards / multi-device sync** — needs a backend. Cool, but rebuilds the architecture. Revisit only if we genuinely want a social layer.
- **AI-generated custom categories on demand** — neat, but blows out Anthropic cost predictability on the worker side. Static daily regeneration is the better cost profile.

---

_Last updated: 2026-04-20_
