# Multi-Date Custom Sessions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow `Customize -> Source -> By date` to select multiple quiz days and build one session pool across all selected days.

**Architecture:** Keep landing-page run selection single-select via `currentRun`. Add `customConfig.dates` as customize-only state, render date pills as multi-select toggles in `By date` mode, and generate custom session `{ category, date }` pairs from the selected date set. Category availability becomes the union of categories present across the selected days, and an empty category selection means all categories across those selected days.

**Tech Stack:** Static HTML/CSS/vanilla JavaScript, `runs.json`, per-run `manifest.json`, gzip-compressed quiz JSON, browser `localStorage`.

---

### Task 1: Add Customize-Only Date Selection State

**Files:**
- Modify: `docs/app.js`

**Step 1: Extend custom config**

Add `dates: []` to `customConfig` and normalize persisted values in `loadState()`.

**Step 2: Add date helper functions**

Introduce helpers that:
- return the effective selected customize dates
- keep at least one selected date in `By date`
- seed stale/empty selections from `currentRun`

**Step 3: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 2: Render Multi-Select Date Pills In Customize

**Files:**
- Modify: `docs/app.js`
- Modify: `docs/style.css`

**Step 1: Reuse the inline customize date row**

Keep the top run picker hidden while Customize is open. In `By date`, render the inline date pills from `customConfig.dates` as toggles instead of binding them to `currentRun`.

**Step 2: Update copy**

Make the source note and category hint talk about selected dates rather than a single current date.

**Step 3: Preserve styles**

Reuse the existing pill styling; only add CSS if the inline layout needs spacing adjustments.

**Step 4: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 3: Build Pools Across Selected Dates

**Files:**
- Modify: `docs/app.js`

**Step 1: Make category availability mode-aware**

For `By date`, compute category ids from the union of all selected manifests instead of only `currentRun`.

**Step 2: Generate category/date pairs**

Update `buildCustomCategoryRunPairs()` so `By date` expands selected categories across every selected date. If categories are empty, include all available categories across the selected dates.

**Step 3: Keep validation aligned**

Filter stale category selections against the union set before building the session.

**Step 4: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 4: Reflect Multi-Date Sessions In Status

**Files:**
- Modify: `docs/app.js`

**Step 1: Keep saved config explicit**

Persist the selected `dates` array in `startSession(..., meta.config)`.

**Step 2: Fix scope summary**

Update `showSessionScope()` so “All categories” is computed against the saved date set for `By date`, not only the active `currentRun`.

**Step 3: Syntax check**

Run: `node --check docs/app.js`
Expected: exit 0.

---

### Task 5: Verify In Browser

**Files:**
- Modify: `docs/index.html` if asset versioning changes are needed

**Step 1: Serve the docs app**

Run: `python3 -m http.server 8081` from `docs/`
Expected: local preview starts.

**Step 2: Verify behavior**

Check:
- `By date` allows selecting multiple days
- deselecting down to zero dates is blocked
- categories reflect the union of selected days
- starting a custom session works
- `Status` shows the correct date count

**Step 3: Final verification**

Run: `node --check docs/app.js`
Expected: exit 0.
