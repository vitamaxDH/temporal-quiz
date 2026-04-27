# Category-First Sessions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support both existing date-based quiz sessions and new category-based archive sessions where a user can focus on one category across all historical quiz days.

**Architecture:** Keep the existing run picker and current-run behavior as the date-first path. Add a `customConfig.sourceMode` switch with `date` and `category` modes. In `date` mode, custom sessions fetch selected categories from `currentRun`; in `category` mode, custom sessions build a pool by fetching each selected category from every archived run where that category appears.

**Tech Stack:** Static HTML/CSS/vanilla JavaScript, gzip-compressed quiz JSON, `runs.json`, per-run `manifest.json`, browser `localStorage`.

---

### Task 1: Replace Per-Category Date Selection With Source Mode State

**Files:**
- Modify: `docs/app.js`

**Step 1: Remove the incorrect per-category date state**

Remove `customConfig.categoryRuns` and the migration block that initializes it:

```js
let customConfig = {
  sourceMode: 'date',
  categories: [],
  difficulties: [],
  length: 10,
};
```

In `loadState()`, normalize `sourceMode`:

```js
if (customConfig.sourceMode !== 'category') customConfig.sourceMode = 'date';
```

**Step 2: Keep the useful historical run manifest cache**

Keep these helper concepts from the current work:

```js
let runManifestCache = {};

function runQuizPath(runDate, suffix) {
  return `quizzes/runs/${runDate}/${suffix}`;
}

async function fetchRunManifest(runDate) { /* existing helper */ }
async function ensureRunManifests() { /* existing helper */ }
```

**Step 3: Remove the wrong selectors**

Delete:
- `getSelectedRunDatesForCategory`
- `setCategoryRunDates`
- `toggleCategoryRunDate`
- `toggleAllCategoryRunDates`
- `formatCategoryRunSummary`
- `renderCustomCategoryRuns`

**Step 4: Syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

---

### Task 2: Add Category Archive Helpers

**Files:**
- Modify: `docs/app.js`

**Step 1: Add mode-aware category helpers**

Add:

```js
function getCurrentRunCategoryIds() {
  return new Set((manifest?.categories ?? []).map(c => c.category));
}

function getArchiveCategoryIds() {
  const ids = new Set();
  getKnownRunDates().forEach(date => {
    const m = runManifestCache[date] || (date === currentRun ? manifest : null);
    (m?.categories ?? []).forEach(entry => ids.add(entry.category));
  });
  return ids;
}

function getCustomCategoryIds() {
  return customConfig.sourceMode === 'category'
    ? getArchiveCategoryIds()
    : getCurrentRunCategoryIds();
}

function getRunDatesForCategory(category) {
  return getKnownRunDates().filter(date => {
    const m = runManifestCache[date] || (date === currentRun ? manifest : null);
    return (m?.categories ?? []).some(entry => entry.category === category);
  });
}
```

**Step 2: Ensure category mode has manifest data before rendering**

When opening Customize, render immediately with cached/current data, then call `ensureRunManifests()` and re-render only if the user is still on Customize:

```js
ensureRunManifests().then(() => {
  if (mode === 'landing' && document.getElementById('customCats')) {
    renderCustomCategories();
    renderCustomSourceMode();
  }
});
```

**Step 3: Syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

---

### Task 3: Redesign Customize UI Around “Source”

**Files:**
- Modify: `docs/app.js`
- Modify: `docs/style.css`

**Step 1: Replace the wrong “Quiz dates” row**

Remove this row from `openCustomize()`:

```html
<div class="customize-row">
  <div class="customize-label">Quiz dates <span class="customize-hint">(for selected categories)</span></div>
  <div class="customize-grid" id="customCategoryRuns"></div>
</div>
```

Add this row above Categories:

```html
<div class="customize-row">
  <div class="customize-label">Source</div>
  <div class="customize-grid custom-source-grid" id="customSourceMode"></div>
  <div class="customize-source-note" id="customSourceNote"></div>
</div>
```

**Step 2: Add a source-mode renderer**

Add:

```js
function renderCustomSourceMode() {
  const host = document.getElementById('customSourceMode');
  const note = document.getElementById('customSourceNote');
  if (!host) return;
  host.innerHTML = '';

  [
    { value: 'date', label: 'By date' },
    { value: 'category', label: 'By category' },
  ].forEach(opt => {
    const pill = document.createElement('button');
    pill.type = 'button';
    pill.className = 'pill';
    if (customConfig.sourceMode === opt.value) pill.classList.add('active');
    pill.textContent = opt.label;
    pill.addEventListener('click', () => {
      customConfig.sourceMode = opt.value;
      customConfig.categories = [];
      saveState();
      renderCustomSourceMode();
      renderCustomCategories();
    });
    host.appendChild(pill);
  });

  if (note) {
    note.textContent = customConfig.sourceMode === 'category'
      ? 'Builds a focused pool from selected categories across every archived quiz day.'
      : `Builds a pool from the selected categories in ${currentRun ? formatRunDate(currentRun) : 'the current run'}.`;
  }
}
```

**Step 3: Update category rendering**

Change `renderCustomCategories()` to use:

```js
const manifestIds = getCustomCategoryIds();
```

For the category mode empty hint, change the category label hint to:

```js
const hint = customConfig.sourceMode === 'category'
  ? '(pick one or more topics)'
  : '(empty = all in current date)';
```

Keep the category groups and All toggles, but in category mode the category set comes from all archived manifests.

**Step 4: Add CSS for the source note**

Add:

```css
.customize-source-note {
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.45;
}
.custom-source-grid {
  gap: 8px;
}
```

Remove stale CSS for:
- `#customCategoryRuns`
- `.customize-date-group`
- `.customize-date-label`
- `.pill-date`
- `.customize-date-empty`

**Step 5: Syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

---

### Task 4: Build Custom Sessions by Date or by Category

**Files:**
- Modify: `docs/app.js`

**Step 1: Add a pair builder**

Add:

```js
function buildCustomCategoryRunPairs() {
  if (customConfig.sourceMode === 'category') {
    const cats = customConfig.categories;
    return cats.flatMap(cat =>
      getRunDatesForCategory(cat).map(date => ({ cat, date }))
    );
  }

  const cats = customConfig.categories.length > 0
    ? customConfig.categories
    : (manifest?.categories?.map(c => c.category) ?? []);
  return cats.map(cat => ({ cat, date: currentRun }));
}
```

**Step 2: Require categories in category mode**

At the start of `startCustomSession()`:

```js
if (customConfig.sourceMode === 'category') {
  await ensureRunManifests();
  if (customConfig.categories.length === 0) {
    openCustomize();
    showCustomizeEmpty('Pick at least one category for category mode.');
    return;
  }
}
```

Extract the existing empty-message behavior into:

```js
function showCustomizeEmpty(message) {
  const host = document.querySelector('.customize');
  if (!host) return;
  const existing = host.querySelector('.customize-empty');
  if (existing) existing.remove();
  const msg = document.createElement('div');
  msg.className = 'customize-empty';
  msg.textContent = message;
  host.appendChild(msg);
}
```

**Step 3: Fetch using pairs**

Replace the old `chosenCats` fetch logic with:

```js
const categoryRunPairs = buildCustomCategoryRunPairs();
const settled = await Promise.all(categoryRunPairs.map(async ({ cat, date }) => {
  try {
    return await fetchCategoryQuestionsForRun(cat, date);
  } catch (e) {
    console.error('Failed to load category for custom session', cat, date, e);
    return [];
  }
}));
```

**Step 4: Store mode and date coverage in session config**

Create:

```js
const dates = Array.from(new Set(categoryRunPairs.map(pair => pair.date)));
const categories = Array.from(new Set(categoryRunPairs.map(pair => pair.cat)));
```

Pass:

```js
config: {
  sourceMode: customConfig.sourceMode,
  categories,
  dates,
  difficulties: [...customConfig.difficulties],
  length: customConfig.length,
}
```

**Step 5: Syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

---

### Task 5: Update Session Status Text

**Files:**
- Modify: `docs/app.js`

**Step 1: Make mode label explicit**

In `showSessionScope()`, use:

```js
const modeLabel = sess?.config?.sourceMode === 'category'
  ? 'Custom · by category'
  : sess?.mode === 'custom'
    ? 'Custom · by date'
    : 'Quick start';
```

**Step 2: Show date coverage**

Replace the current date display logic with:

```js
const dates = Array.isArray(sess?.config?.dates) && sess.config.dates.length
  ? sess.config.dates
  : (currentRun ? [currentRun] : []);

const datesDisplay = dates.length === 1
  ? formatRunDate(dates[0])
  : `${dates.length} quiz dates`;
```

Then keep rendering `Dates`.

**Step 3: Syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

---

### Task 6: Final Verification

**Files:**
- Verify only.

**Step 1: Run JavaScript syntax check**

Run:

```bash
node --check docs/app.js
```

Expected: exit 0.

**Step 2: Run Go tests**

Run:

```bash
cd worker && go test ./... -count=1
```

Expected: all packages pass.

**Step 3: Check diff whitespace**

Run:

```bash
git diff --check
```

Expected: no output, exit 0.

**Step 4: Verify static server assets**

If the local server is running:

```bash
curl -s 'http://127.0.0.1:8080/app.js?v=74' | rg 'sourceMode|buildCustomCategoryRunPairs|getRunDatesForCategory'
curl -s 'http://127.0.0.1:8080/style.css?v=74' | rg 'customize-source-note|custom-source-grid'
```

Expected: both commands find the new symbols.

**Step 5: Manual UI verification**

Open `http://127.0.0.1:8080/`.

Verify:
- Date picker still switches the active daily run.
- Customize → Source → By date behaves like the old current-run custom flow.
- Customize → Source → By category → Workflows builds a pool from all archived days containing `Features_Workflows`.
- Session Status shows `Custom · by category` and multiple quiz dates.

---
