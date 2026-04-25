# Customize Filter Counts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show quiz counts next to each difficulty filter and the `∞` length option in Customize, while renaming the displayed `med` difficulty label to `medium`.

**Architecture:** Add a small in-memory question cache keyed by `{runDate, category}` and compute customize stats from the current source/date/category selection before difficulty filtering. Render difficulty chips from that base-pool count snapshot, render `∞` using the currently available post-difficulty pool size, and route all visible difficulty labels through one formatter that maps `med -> medium`.

**Tech Stack:** Static HTML/CSS/vanilla JavaScript, gzip quiz JSON, browser `localStorage`, in-memory module caches.

---

### Task 1: Normalize Difficulty Display Labels

**Files:**
- Modify: `docs/app.js`

**Step 1: Add a display formatter**

Add a helper that maps internal difficulty ids to display labels:

```js
function formatDifficultyLabel(difficulty) {
  return difficulty === 'med' ? 'medium' : (difficulty || 'easy');
}
```

**Step 2: Use it everywhere visible**

Update `difficultyDots()` and status text rendering so `medium` is shown instead of `med` while keeping the stored data shape unchanged.

**Step 3: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 2: Add Reusable Customize Pool Stats

**Files:**
- Modify: `docs/app.js`

**Step 1: Add a per-session question cache**

Cache fetched category question payloads by run date + category so customize count recomputation does not refetch the same quiz files repeatedly.

**Step 2: Add stats helpers**

Create helpers to:
- build the current customize category/date pair list
- fetch the base pool for current source/date/category selection
- count totals by difficulty
- count the post-difficulty filtered pool

**Step 3: Guard async re-renders**

Use a sequence/token so old async count responses do not overwrite newer customize selections after rapid clicking.

**Step 4: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 3: Render Counts In Difficulty And Length Controls

**Files:**
- Modify: `docs/app.js`
- Modify: `docs/style.css`

**Step 1: Update difficulty chips**

Render labels like:
- `All (120)`
- `easy (20)`
- `medium (40)`
- `hard (40)`
- `nightmare (20)`

These counts must come from the base pool before difficulty filtering.

**Step 2: Update length options**

Keep `5`, `10`, `20` unchanged, but render `∞ (N)` where `N` is the currently available pool after the difficulty filter.

**Step 3: Refresh counts on relevant changes**

Recompute counts when source mode, selected dates, categories, or difficulties change, and when Customize first opens.

**Step 4: Keep styles compact**

If the longer count labels wrap awkwardly, add a small gap/line-height adjustment in CSS without changing the existing look more than necessary.

**Step 5: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 4: Verify In Browser

**Files:**
- Modify: `docs/index.html` if asset versioning changes are needed

**Step 1: Serve the docs app**

Run: `python3 -m http.server 8081` from `docs/`
Expected: local preview starts.

**Step 2: Verify behavior**

Check:
- difficulty chips show stable base-pool counts
- selecting `easy` keeps the chips showing the original base counts
- `∞` updates to the post-difficulty pool size
- `medium` is visible instead of `med`
- counts update when categories/dates/source change

**Step 3: Final verification**

Run: `node --check docs/app.js`
Expected: exit 0.
