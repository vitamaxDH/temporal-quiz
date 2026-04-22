# Quiz UI Improvements: Sidebar History + Sorted Categories

**Goal:** Make the quiz page feel more polished and useful. Three changes:
1. Sort category pills alphabetically
2. Add a sidebar showing past quiz question history (answered questions with results)
3. General visual cleanup

---

## 1. Sort Category Pills

Currently pills render in whatever order the manifest returns them. Sort alphabetically in `renderCategories()`.

**File:** `web/app.js`

```javascript
// In renderCategories()
const sorted = [...manifest.categories].sort((a, b) =>
  a.category.localeCompare(b.category)
);
sorted.forEach(cat => { ... });
```

One-line change. No new files.

---

## 2. Sidebar with Quiz History

### Concept

A slide-out sidebar on the right side. Shows:
- List of recently answered questions (stored in localStorage)
- Each entry: category pill, question snippet (truncated), correct/wrong indicator, difficulty dots
- Clicking an entry shows that question again (review mode)
- Toggle button in the header or stats bar

### Data Model

Extend the existing `state` in localStorage to track answered questions:

```javascript
state = {
  streak: 0,
  totalAnswered: 0,
  totalCorrect: 0,
  categoryStats: {},
  history: []  // NEW: array of { question, answer, selectedKey, correct, category, difficulty, timestamp }
};
```

Cap at 200 entries (FIFO). Each entry is ~500 bytes, so 200 entries = ~100KB in localStorage. Fine.

### HTML Changes (`index.html`)

Add a sidebar toggle button in the header and a sidebar panel:

```html
<div class="header">
  <h1>temporal.quiz</h1>
  <div class="tagline">test your workflow knowledge</div>
  <button class="btn btn-ghost btn-sidebar-toggle" onclick="toggleSidebar()">History</button>
</div>

<div class="sidebar" id="sidebar">
  <div class="sidebar-header">
    <h2>History</h2>
    <button class="btn btn-ghost" onclick="toggleSidebar()">Close</button>
  </div>
  <div class="sidebar-list" id="historyList"></div>
</div>
```

### CSS Changes (`style.css`)

```css
/* Sidebar */
.sidebar {
  position: fixed;
  top: 0;
  right: -360px;
  width: 360px;
  height: 100vh;
  background: var(--bg-card);
  border-left: 1px solid var(--border);
  z-index: 40;
  transition: right 0.3s ease;
  overflow-y: auto;
  padding: 24px;
}
.sidebar.open {
  right: 0;
}
.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.sidebar-header h2 {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-primary);
}
.history-item {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.15s;
}
.history-item:hover {
  background: var(--bg-hover);
}
.history-item.correct {
  border-left: 3px solid var(--correct);
}
.history-item.wrong {
  border-left: 3px solid var(--wrong);
}
.history-question {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
  margin-top: 4px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.history-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
}
```

### JS Changes (`app.js`)

```javascript
// In submitAnswer(), after updating state:
state.history.unshift({
  question: q.question.substring(0, 150),
  category: q.category,
  difficulty: q.difficulty,
  correct: isCorrect,
  selectedKey: selectedKey,
  answer: q.answer,
  timestamp: Date.now()
});
if (state.history.length > 200) state.history.pop();

// Toggle sidebar
function toggleSidebar() {
  const sidebar = document.getElementById('sidebar');
  sidebar.classList.toggle('open');
  if (sidebar.classList.contains('open')) renderHistory();
}

// Render history list
function renderHistory() {
  const list = document.getElementById('historyList');
  const items = state.history || [];
  if (items.length === 0) {
    list.innerHTML = '<p style="color:var(--text-muted);font-size:12px;">No questions answered yet.</p>';
    return;
  }
  list.innerHTML = items.map(h => `
    <div class="history-item ${h.correct ? 'correct' : 'wrong'}">
      <div class="history-meta">
        <span>${formatCategoryLabel(h.category)}</span>
        <span>${difficultyDots(h.difficulty)}</span>
      </div>
      <div class="history-question">${h.question}</div>
      <div class="history-meta" style="margin-top:6px;">
        <span>${h.correct ? 'Correct' : 'Wrong'} (${h.selectedKey}→${h.answer})</span>
        <span>${new Date(h.timestamp).toLocaleDateString()}</span>
      </div>
    </div>
  `).join('');
}
```

---

## 3. Files Changed

| File | Changes |
|------|---------|
| `web/app.js` | Sort categories, add history tracking in submitAnswer, add toggleSidebar/renderHistory |
| `web/index.html` | Add sidebar HTML, add History button in header |
| `web/style.css` | Add sidebar and history item styles |

---

## Verification

- `python3 -m http.server 8080` from `web/`
- Calculator button works (already fixed)
- Category pills sorted alphabetically
- Answer a few questions, open History sidebar, see them listed
- Sidebar slides in/out smoothly
- History persists across page reloads (localStorage)
