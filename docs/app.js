/* temporal.quiz - Terminal Elegance quiz application */

const STORAGE_KEY = 'temporal-quiz-state';
const CIRCUMFERENCE = 2 * Math.PI * 15; // 94.25 for r=15

let manifest = null;
let questions = [];
let currentIndex = 0;
let selectedKey = null;
let revealed = false;
let currentCategory = null;
let currentDifficulty = null; // null = all difficulties

// State persisted to localStorage
let state = {
  streak: 0,
  totalAnswered: 0,
  totalCorrect: 0,
  categoryStats: {}, // { category: { answered: N, correct: N } }
  history: []         // [{ question, category, difficulty, correct, selectedKey, answer, timestamp }]
};

/* ---- Persistence ---- */

function loadState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      state = { ...state, ...parsed };
    }
  } catch (e) {
    console.warn('Failed to load state from localStorage', e);
  }
}

function saveState() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch (e) {
    console.warn('Failed to save state to localStorage', e);
  }
}

/* ---- Shuffle ---- */

function shuffle(arr) {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/* ---- Format helpers ---- */

function formatQuestion(text) {
  // Fenced code blocks: ```lang\n...\n``` -> <pre><code>
  text = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
    const escaped = code.replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return `<pre class="code-block" data-lang="${lang}"><code>${escaped}</code></pre>`;
  });
  // Inline backticks: `code` -> <code>
  text = text.replace(/`([^`]+)`/g, '<code>$1</code>');
  // Newlines to <br> (outside of <pre> blocks)
  text = text.replace(/\n/g, '<br>');
  return text;
}

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

function formatCategoryLabel(category) {
  return category.replace(/_/g, ' ').replace(/ and /g, ' & ');
}

/* Turn a source_doc filename into a readable sub-topic.
   Examples:
     parent-close-policy.html              -> "Parent Close Policy"
     nexus_endpoints.html                  -> "Endpoints"
     develop_dotnet_activities_async.html  -> "Async"
   The category chip already gives broader context, so we keep
   only the last underscore-separated segment for brevity. */
function formatSubcategory(sourceDoc) {
  if (!sourceDoc) return '';
  let name = sourceDoc.replace(/\.html?$/i, '');
  const parts = name.split('_');
  const leaf = parts[parts.length - 1] || name;
  if (!leaf || leaf.toLowerCase() === 'index') return '';
  return leaf
    .split('-')
    .filter(Boolean)
    .map(w => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/* ---- Category groups ----
   Order matters: more important groups come first.
   Each entry can be a string (uses default label) or
   { id, label } to override the in-group display label. */

const CATEGORY_GROUPS = [
  {
    label: 'Features',
    categories: [
      { id: 'Features_Workflows',              label: 'Workflows' },
      { id: 'Features_Activities',             label: 'Activities' },
      { id: 'Features_Workers_and_Routing',    label: 'Workers & Routing' },
      { id: 'Features_Messaging_and_Visibility', label: 'Messaging & Visibility' },
      { id: 'Features_Data_and_Security',      label: 'Data & Security' },
      { id: 'Features_Nexus',                  label: 'Nexus' },
      { id: 'Tags',                            label: 'Tags' },
      { id: 'Features_Other',                  label: 'Other' },
    ],
  },
  {
    label: 'Develop (SDKs)',
    categories: [
      { id: 'Develop',            label: 'Develop' },
      { id: 'Develop_General',    label: 'General' },
      { id: 'Develop_Go',         label: 'Go' },
      { id: 'Develop_Java',       label: 'Java' },
      { id: 'Develop_Python',     label: 'Python' },
      { id: 'Develop_TypeScript', label: 'TypeScript' },
      { id: 'Develop_Other_SDKs', label: 'Other SDKs' },
    ],
  },
  {
    label: 'Concepts & Tooling',
    categories: [
      { id: 'Evaluate_and_Concepts', label: 'Evaluate & Concepts' },
      { id: 'CLI_and_References',    label: 'CLI & References' },
      { id: 'AI_and_Cookbook',       label: 'AI & Cookbook' },
    ],
  },
  {
    label: 'Operations',
    categories: [
      { id: 'Self_Hosted_and_Ops', label: 'Self Hosted & Ops' },
      { id: 'Temporal_Cloud',      label: 'Temporal Cloud' },
    ],
  },
  {
    label: 'Other',
    categories: [
      { id: 'Miscellaneous', label: 'Miscellaneous' },
    ],
  },
];

/* ---- Init ---- */

async function init() {
  loadState();
  updateStats();

  // Wire up buttons
  document.getElementById('skipBtn').addEventListener('click', skipQuestion);
  document.getElementById('submitBtn').addEventListener('click', () => {
    if (revealed) { nextQuestion(); } else { submitAnswer(); }
  });
  document.getElementById('historyBtn').addEventListener('click', toggleSidebar);
  document.getElementById('sidebarCloseBtn').addEventListener('click', toggleSidebar);
  document.getElementById('sidebarOverlay').addEventListener('click', toggleSidebar);
  document.getElementById('calcToggleBtn').addEventListener('click', toggleCalculator);
  document.getElementById('calcEvalBtn').addEventListener('click', calcEval);
  document.getElementById('calcClearBtn').addEventListener('click', calcClear);
  document.getElementById('calcButtons').addEventListener('click', (e) => {
    if (e.target.dataset.calc) calcInput(e.target.dataset.calc);
  });

  try {
    const res = await fetch('quizzes/manifest.json');
    manifest = await res.json();
    renderCategories();
    renderDifficultyFilter();
  } catch (e) {
    console.error('Failed to load manifest', e);
  }
}

/* ---- Categories ---- */

function renderCategories() {
  const container = document.querySelector('.categories');
  container.innerHTML = '';

  // Index manifest categories for quick lookup + uncategorized detection.
  const manifestIds = new Set(manifest.categories.map(c => c.category));
  const groupedIds = new Set();
  CATEGORY_GROUPS.forEach(g => g.categories.forEach(c => groupedIds.add(c.id)));

  // "All" pill — its own row at the top.
  const allRow = document.createElement('div');
  allRow.className = 'category-row category-row-all';
  const allPill = document.createElement('div');
  allPill.className = 'pill pill-all';
  allPill.textContent = 'All';
  allPill.addEventListener('click', () => loadCategory('all'));
  allRow.appendChild(allPill);
  container.appendChild(allRow);

  // Render each group as a labeled section.
  CATEGORY_GROUPS.forEach(group => {
    const visible = group.categories.filter(c => manifestIds.has(c.id));
    if (visible.length === 0) return;

    const groupEl = document.createElement('div');
    groupEl.className = 'category-group';

    const labelEl = document.createElement('div');
    labelEl.className = 'category-group-label';
    labelEl.textContent = group.label;
    groupEl.appendChild(labelEl);

    const rowEl = document.createElement('div');
    rowEl.className = 'category-row';
    visible.forEach(cat => {
      const pill = document.createElement('div');
      pill.className = 'pill';
      pill.textContent = cat.label || formatCategoryLabel(cat.id);
      pill.dataset.category = cat.id;
      pill.title = formatCategoryLabel(cat.id);
      pill.addEventListener('click', () => loadCategory(cat.id));
      rowEl.appendChild(pill);
    });
    groupEl.appendChild(rowEl);
    container.appendChild(groupEl);
  });

  // Catch-all: anything in the manifest that isn't grouped yet.
  const ungrouped = manifest.categories
    .filter(c => !groupedIds.has(c.category))
    .sort((a, b) => a.category.localeCompare(b.category));
  if (ungrouped.length > 0) {
    const groupEl = document.createElement('div');
    groupEl.className = 'category-group';

    const labelEl = document.createElement('div');
    labelEl.className = 'category-group-label';
    labelEl.textContent = 'More';
    groupEl.appendChild(labelEl);

    const rowEl = document.createElement('div');
    rowEl.className = 'category-row';
    ungrouped.forEach(cat => {
      const pill = document.createElement('div');
      pill.className = 'pill';
      pill.textContent = formatCategoryLabel(cat.category);
      pill.dataset.category = cat.category;
      pill.addEventListener('click', () => loadCategory(cat.category));
      rowEl.appendChild(pill);
    });
    groupEl.appendChild(rowEl);
    container.appendChild(groupEl);
  }
}

function setActivePill(category) {
  // Only clear category pills, not difficulty pills
  document.querySelectorAll('.categories .pill').forEach(p => p.classList.remove('active'));
  if (category === 'all') {
    document.querySelector('.categories .pill').classList.add('active');
  } else {
    const pill = document.querySelector(`.categories .pill[data-category="${category}"]`);
    if (pill) pill.classList.add('active');
  }
}

/* ---- Difficulty filter ---- */

const DIFFICULTIES = ['easy', 'med', 'hard', 'nightmare'];

function renderDifficultyFilter() {
  const container = document.querySelector('.difficulty-filter');
  if (!container) return;
  container.innerHTML = '';

  const allPill = document.createElement('div');
  allPill.className = 'pill pill-diff active';
  allPill.textContent = 'All';
  allPill.addEventListener('click', () => setDifficulty(null));
  container.appendChild(allPill);

  DIFFICULTIES.forEach(d => {
    const pill = document.createElement('div');
    pill.className = 'pill pill-diff';
    pill.dataset.difficulty = d;
    pill.innerHTML = difficultyDots(d);
    pill.addEventListener('click', () => setDifficulty(d));
    container.appendChild(pill);
  });
}

function setDifficulty(difficulty) {
  currentDifficulty = difficulty;

  document.querySelectorAll('.difficulty-filter .pill').forEach(p => p.classList.remove('active'));
  if (difficulty === null) {
    document.querySelector('.difficulty-filter .pill').classList.add('active');
  } else {
    const pill = document.querySelector(`.difficulty-filter .pill[data-difficulty="${difficulty}"]`);
    if (pill) pill.classList.add('active');
  }

  // Reload current category with new filter
  if (currentCategory) loadCategory(currentCategory);
}

function filterByDifficulty(qs) {
  if (!currentDifficulty) return qs;
  return qs.filter(q => q.difficulty === currentDifficulty);
}

/* ---- Load questions ---- */

async function fetchCategoryQuestions(category) {
  const res = await fetch(`quizzes/${category}.json`);
  const data = await res.json();
  return data.questions || [];
}

async function loadCategory(category) {
  currentCategory = category;
  setActivePill(category);

  try {
    if (category === 'all') {
      // Load all categories with weak-area weighting
      const allQuestions = [];
      for (const cat of manifest.categories) {
        const catQuestions = await fetchCategoryQuestions(cat.category);
        const catStats = state.categoryStats[cat.category];
        const accuracy = catStats && catStats.answered > 0
          ? catStats.correct / catStats.answered
          : 1;
        // Categories below 70% accuracy get 2x weight
        if (accuracy < 0.7) {
          allQuestions.push(...catQuestions, ...catQuestions);
        } else {
          allQuestions.push(...catQuestions);
        }
      }
      questions = shuffle(filterByDifficulty(allQuestions));
    } else {
      const catQuestions = await fetchCategoryQuestions(category);
      questions = shuffle(filterByDifficulty(catQuestions));
    }

    currentIndex = 0;
    if (questions.length > 0) {
      showQuestion();
    } else {
      document.querySelector('.question-card').innerHTML =
        '<p class="question-text">No questions match this filter.</p>';
      document.querySelector('.answers').innerHTML = '';
      document.getElementById('explanation').style.display = 'none';
    }
  } catch (e) {
    console.error('Failed to load category', category, e);
  }
}

/* ---- Display question ---- */

function showQuestion() {
  if (!questions.length) return;

  const q = questions[currentIndex];
  revealed = false;
  selectedKey = null;

  // Question card
  const card = document.querySelector('.question-card');
  const subcategory = formatSubcategory(q.source_doc);
  const subEl = subcategory
    ? `<span class="question-subcategory">${subcategory}</span>`
    : '';
  card.innerHTML = `
    <div class="question-meta">
      <div class="question-meta-left">
        <span class="question-category">${formatCategoryLabel(q.category)}</span>
        ${subEl}
      </div>
      <span class="question-difficulty">${difficultyDots(q.difficulty)}</span>
    </div>
    <p class="question-text">${formatQuestion(q.question)}</p>
  `;
  // Re-trigger card animation
  card.style.animation = 'none';
  card.offsetHeight; // force reflow
  card.style.animation = '';

  // Answers
  const answersEl = document.querySelector('.answers');
  answersEl.innerHTML = '';
  q.choices.forEach(choice => {
    const div = document.createElement('div');
    div.className = 'answer';
    div.innerHTML = `
      <div class="answer-key">${choice.key}</div>
      <div class="answer-text">${formatQuestion(choice.text)}</div>
    `;
    div.addEventListener('click', () => selectAnswer(choice.key));
    answersEl.appendChild(div);
  });

  // Hide explanation
  const explanation = document.getElementById('explanation');
  explanation.style.display = 'none';
  explanation.innerHTML = '';

  // Reset submit button
  const btn = document.getElementById('submitBtn');
  btn.textContent = 'Submit';
  btn.disabled = true;

  updateProgress();
}

/* ---- Answer selection ---- */

function selectAnswer(key) {
  if (revealed) return;

  selectedKey = key;
  document.querySelectorAll('.answer').forEach(a => {
    const k = a.querySelector('.answer-key').textContent.trim();
    if (k === key) {
      a.classList.add('selected');
    } else {
      a.classList.remove('selected');
    }
  });

  document.getElementById('submitBtn').disabled = false;
}

/* ---- Submit ---- */

function submitAnswer() {
  if (!selectedKey || revealed) return;
  revealed = true;

  const q = questions[currentIndex];
  const isCorrect = selectedKey === q.answer;

  // Update state
  state.totalAnswered++;
  if (isCorrect) {
    state.totalCorrect++;
    state.streak++;
  } else {
    state.streak = 0;
  }

  // Category stats
  if (!state.categoryStats[q.category]) {
    state.categoryStats[q.category] = { answered: 0, correct: 0 };
  }
  state.categoryStats[q.category].answered++;
  if (isCorrect) {
    state.categoryStats[q.category].correct++;
  }

  // History
  if (!state.history) state.history = [];
  state.history.unshift({
    question: q.question.substring(0, 150),
    category: q.category,
    subcategory: formatSubcategory(q.source_doc),
    difficulty: q.difficulty,
    correct: isCorrect,
    selectedKey,
    answer: q.answer,
    timestamp: Date.now()
  });
  if (state.history.length > 200) state.history.pop();

  saveState();
  updateStats();

  // Reveal answers
  document.querySelectorAll('.answer').forEach(a => {
    const k = a.querySelector('.answer-key').textContent.trim();
    a.classList.remove('selected');
    if (k === q.answer) {
      a.classList.add('correct');
    } else if (k === selectedKey && !isCorrect) {
      a.classList.add('wrong');
    } else {
      a.classList.add('dimmed');
    }
  });

  // Show explanation
  const explanation = document.getElementById('explanation');
  explanation.style.display = 'block';
  explanation.innerHTML = `
    <div class="explanation-label">Explanation</div>
    <div class="explanation-text">${formatQuestion(q.explanation)}</div>
  `;

  // Switch button to Next
  const btn = document.getElementById('submitBtn');
  btn.textContent = 'Next';
  btn.disabled = false;
}

/* ---- Next / Skip ---- */

function nextQuestion() {
  currentIndex++;
  if (currentIndex >= questions.length) {
    // Reshuffle and restart
    questions = shuffle(questions);
    currentIndex = 0;
  }
  showQuestion();
}

function skipQuestion() {
  nextQuestion();
}

/* ---- Stats / Progress ---- */

function updateProgress() {
  const total = questions.length;
  const current = Math.min(currentIndex + 1, total);
  const fraction = total > 0 ? current / total : 0;
  const offset = CIRCUMFERENCE * (1 - fraction);

  document.querySelector('.progress-ring .fill').style.strokeDashoffset = offset;
  document.querySelector('.progress-ring .text').textContent = `${current}/${total}`;
}

function updateStats() {
  document.querySelector('.stat-value.streak').textContent = state.streak;
  const accuracy = state.totalAnswered > 0
    ? Math.round((state.totalCorrect / state.totalAnswered) * 100)
    : 0;
  document.querySelector('.stat-value.accuracy').textContent = accuracy + '%';
}

/* ---- Keyboard shortcuts ---- */

document.addEventListener('keydown', (e) => {
  // Ignore if user is typing in an input
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

  const key = e.key.toUpperCase();
  if (['A', 'B', 'C', 'D'].includes(key) && !revealed) {
    selectAnswer(key);
  } else if (e.key === 'Enter') {
    if (revealed) {
      nextQuestion();
    } else if (selectedKey) {
      submitAnswer();
    }
  }
});

/* ---- Sidebar / History ---- */

function toggleSidebar() {
  document.getElementById('sidebar').classList.toggle('open');
  document.getElementById('sidebarOverlay').classList.toggle('open');
  if (document.getElementById('sidebar').classList.contains('open')) {
    renderHistory();
  }
}

function renderHistory() {
  const list = document.getElementById('historyList');
  const items = state.history || [];

  if (items.length === 0) {
    list.innerHTML = '<p class="history-empty">No questions answered yet.</p>';
    return;
  }

  list.innerHTML = items.map(h => {
    const sub = h.subcategory ? ` &middot; ${h.subcategory}` : '';
    return `
    <div class="history-item ${h.correct ? 'correct' : 'wrong'}">
      <div class="history-meta">
        <span>${formatCategoryLabel(h.category)}${sub}</span>
        <span>${difficultyDots(h.difficulty)}</span>
      </div>
      <div class="history-question">${h.question}</div>
      <div class="history-result ${h.correct ? 'correct' : 'wrong'}">
        ${h.correct ? 'Correct' : 'Wrong (' + h.selectedKey + ' instead of ' + h.answer + ')'}
      </div>
    </div>
  `;
  }).join('');
}

/* ---- Calculator ---- */

function toggleCalculator() {
  const calc = document.getElementById('calculator');
  calc.style.display = calc.style.display === 'none' ? 'block' : 'none';
}

function calcInput(val) {
  document.getElementById('calcDisplay').value += val;
}

function calcClear() {
  document.getElementById('calcDisplay').value = '';
}

function calcEval() {
  const display = document.getElementById('calcDisplay');
  try {
    const sanitized = display.value.replace(/[^0-9+\-*/.() ]/g, '');
    display.value = Function('"use strict"; return (' + sanitized + ')')();
  } catch {
    display.value = 'Error';
  }
}

/* ---- Boot ---- */

init();
