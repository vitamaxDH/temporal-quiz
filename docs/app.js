/* temporal.quiz - Terminal Elegance quiz application */

const STORAGE_KEY = 'temporal-quiz-state';
const CIRCUMFERENCE = 2 * Math.PI * 18; // 113.097 for r=18 (44px ring)

let manifest = null;
let questions = [];
let currentIndex = 0;
let selectedKey = null;
let revealed = false;

// Session state
let mode = 'landing';          // 'landing' | 'quiz' | 'recap'
let sessionLength = 10;        // 5 | 10 | 20 | Infinity
let sessionAnswered = 0;
let sessionCorrect = 0;
let sessionWrongItems = []; // [{ question, choices, answer, selectedKey, difficulty, category, source_doc }]

// Custom config (persisted to localStorage via saveState)
let customConfig = {
  categories: [],
  difficulties: [],
  length: 10,
};

// Run history: list of { date, generated_at, total_count } sorted newest-first.
// currentRun is the YYYY-MM-DD of the active run, or null when we're falling
// back to the legacy flat layout (no runs.json on the server).
let runs = [];
let currentRun = null;
const MAX_RUNS_VISIBLE = 7;

// Notes UI state (session-only, not persisted)
let notesActiveCategory = null;

// State persisted to localStorage
let state = {
  streak: 0,
  totalAnswered: 0,
  totalCorrect: 0,
  categoryStats: {}, // { category: { answered: N, correct: N } }
  sessions: [],       // [{ id, started_at, ended_at, mode, config, planned, total, correct, history:[...] }]
  historyResetAt: null, // ms timestamp of last Reset (null = never reset)
  theme: 'dark',      // 'dark' | 'light'
  categoryNotes: {},  // { category: 'markdown text' }
  questionNotes: {},  // { <hash>: { hash, note, question, source_doc, category, created_at, updated_at } }
  toolbox: { x: null, y: null, open: false, tab: 'calc' }
};

/* ---- Persistence ---- */

function loadState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      state = { ...state, ...parsed };
      if (!state.categoryNotes) state.categoryNotes = {};
      if (!state.questionNotes) state.questionNotes = {};
      if (parsed.customConfig) {
        customConfig = { ...customConfig, ...parsed.customConfig };
        if (customConfig.length === null) customConfig.length = Infinity;
      }
    }
  } catch (e) {
    console.warn('Failed to load state from localStorage', e);
  }
}

function saveState() {
  try {
    const blob = { ...state, customConfig };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(blob));
  } catch (e) {
    console.warn('Failed to save state to localStorage', e);
  }
}

/* ---- Theme ---- */

function applyTheme() {
  const theme = state.theme === 'light' ? 'light' : 'dark';
  document.documentElement.dataset.theme = theme;
  const btn = document.getElementById('themeBtn');
  if (btn) {
    // Show the mode you'd switch TO
    btn.innerHTML = theme === 'dark' ? '\u2600' : '\u263E';
    btn.title = theme === 'dark' ? 'Switch to light' : 'Switch to dark';
  }
}

function toggleTheme() {
  state.theme = state.theme === 'light' ? 'dark' : 'light';
  saveState();
  applyTheme();
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

function debounce(fn, ms) {
  let t;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), ms);
  };
}

function normalizeQuestionText(q) {
  return (q || '').trim().toLowerCase().replace(/\s+/g, ' ');
}

function hashQuestion(questionText) {
  // DJB2 hash. Returns 8 hex chars. Sufficient as a dict key for personal use.
  const s = normalizeQuestionText(questionText);
  let h = 5381;
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) + h + s.charCodeAt(i)) | 0;
  }
  return (h >>> 0).toString(16).padStart(8, '0');
}

function escapeHtml(s) {
  return (s || '').replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[c]));
}

/* ---- Format helpers ---- */

function formatQuestion(text) {
  if (text == null) return '';
  // Stash code blocks as placeholders so markdown passes can't corrupt them.
  const parts = [];
  const stash = (html) => {
    const i = parts.length;
    parts.push(html);
    return `\u0001${i}\u0001`;
  };

  // Fenced code blocks first: ```lang\n...\n```
  text = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
    const escaped = code.replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return stash(`<pre class="code-block" data-lang="${lang}"><code>${escaped}</code></pre>`);
  });

  // Inline code next: `code`
  text = text.replace(/`([^`]+)`/g, (_, code) => stash(`<code>${code}</code>`));

  // Bold: **text** (no line breaks, no nested **)
  text = text.replace(/\*\*([^\n*]+)\*\*/g, '<strong>$1</strong>');

  // Italic: _text_ (single underscore, not mid-word like snake_case_id)
  text = text.replace(/(^|[^A-Za-z0-9_])_([^\n_]+?)_(?![A-Za-z0-9_])/g, '$1<em>$2</em>');

  // Newlines to <br> (outside of stashed code blocks)
  text = text.replace(/\n/g, '<br>');

  // Restore code placeholders.
  text = text.replace(/\u0001(\d+)\u0001/g, (_, i) => parts[Number(i)]);

  return text;
}

function difficultyDots(difficulty) {
  const levels = { easy: 1, med: 2, hard: 3, nightmare: 4 };
  const safe = levels[difficulty] ? difficulty : 'easy';
  return `<span class="difficulty-badge difficulty-${safe}"><span class="difficulty-label">${difficulty}</span></span>`;
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
/* Resolve the URL prefix for fetching quiz data. All content lives under
   quizzes/runs/<YYYY-MM-DD>/ and currentRun comes from runs.json. */
function quizPath(suffix) {
  return `quizzes/runs/${currentRun}/${suffix}`;
}

/* Human-friendly label for a YYYY-MM-DD run date.
   Today / Yesterday / "Apr 17". Falls back to the raw date on parse error. */
function formatRunDate(dateStr) {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr || '');
  if (!parts) return dateStr || '';
  const [_, y, m, d] = parts;
  const runDate = new Date(Number(y), Number(m) - 1, Number(d));
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const diffDays = Math.round((today - runDate) / (24 * 3600 * 1000));
  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  return runDate.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

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

/* ---- Init ---- */

async function init() {
  loadState();
  applyTheme();
  updateStats();

  // Wire up buttons
  document.getElementById('themeBtn').addEventListener('click', toggleTheme);
  document.getElementById('skipBtn').addEventListener('click', skipQuestion);
  document.getElementById('prevBtn').addEventListener('click', goPrevQuestion);
  document.getElementById('submitBtn').addEventListener('click', () => {
    if (revealed) { nextQuestion(); } else { submitAnswer(); }
  });
  document.getElementById('historyBtn').addEventListener('click', toggleSidebar);
  document.getElementById('aboutBtn').addEventListener('click', renderAbout);
  document.getElementById('notesBtn').addEventListener('click', toggleNotes);
  document.getElementById('notesCloseBtn').addEventListener('click', toggleNotes);
  document.getElementById('notesSidebarOverlay').addEventListener('click', toggleNotes);
  document.getElementById('notesExportBtn').addEventListener('click', exportNotes);
  document.getElementById('sidebarCloseBtn').addEventListener('click', toggleSidebar);
  document.getElementById('historyResetBtn').addEventListener('click', resetHistory);
  document.getElementById('modalConfirmBtn').addEventListener('click', () => closeModal(true));
  document.getElementById('modalCancelBtn').addEventListener('click', () => closeModal(false));
  document.getElementById('modalBackdrop').addEventListener('click', (e) => {
    if (e.target.id === 'modalBackdrop') closeModal(false);
  });
  document.getElementById('restartBtn').addEventListener('click', restartSession);
  document.getElementById('scopeBtn').addEventListener('click', showSessionScope);
  document.getElementById('quitBtn').addEventListener('click', quitSession);
  document.getElementById('sidebarOverlay').addEventListener('click', toggleSidebar);
  document.getElementById('toolboxLauncher').addEventListener('click', toggleToolbox);
  document.getElementById('toolboxClose').addEventListener('click', closeToolbox);
  document.getElementById('toolboxTabs').addEventListener('click', (e) => {
    const btn = e.target.closest('.toolbox-tab');
    if (btn) setToolboxTab(btn.dataset.tab);
  });
  initToolboxDrag();
  applyToolboxPosition();
  window.addEventListener('resize', applyToolboxPosition);
  if (state.toolbox?.open) openToolbox();

  sweepCompletedUnstamped();

  try {
    await loadRuns();
    await loadManifest();
    renderLanding();
  } catch (e) {
    console.error('Failed to initialize quiz data', e);
  }
}

/* Load the list of available runs. Missing runs.json is not an error —
   it just means the server hasn't been migrated to the snapshot layout
   yet, and we fall back to the flat quizzes/ files. */
async function loadRuns() {
  try {
    const res = await fetch('quizzes/runs.json', { cache: 'no-store' });
    if (!res.ok) {
      runs = [];
      currentRun = null;
      return;
    }
    const data = await res.json();
    runs = Array.isArray(data.runs) ? data.runs : [];
    currentRun = runs.length > 0 ? runs[0].date : null;
  } catch (e) {
    runs = [];
    currentRun = null;
  }
  renderRunPicker();
}

async function loadManifest() {
  const res = await fetch(quizPath('manifest.json'), { cache: 'no-store' });
  manifest = await res.json();
  updateGeneratedBadge();
}

function updateGeneratedBadge() {
  const el = document.getElementById('quizGenerated');
  if (!el) return;
  const ts = manifest?.generated_at;
  if (!ts) { el.textContent = ''; return; }
  const d = new Date(ts);
  if (isNaN(d.getTime())) { el.textContent = ''; return; }
  const datePart = d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  const timePart = d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  el.textContent = `Generated ${datePart} · ${timePart}`;
}

/* ---- Run picker ---- */

function renderRunPicker() {
  const labelEl = document.querySelector('.run-section-label');
  const container = document.querySelector('.run-picker');
  if (!container || !labelEl) return;

  // Hide the picker entirely when there's nothing meaningful to switch to.
  if (runs.length < 2) {
    container.style.display = 'none';
    labelEl.style.display = 'none';
    container.innerHTML = '';
    return;
  }

  container.style.display = '';
  labelEl.style.display = '';
  container.innerHTML = '';

  const visible = runs.slice(0, MAX_RUNS_VISIBLE);
  visible.forEach(run => {
    const pill = document.createElement('div');
    pill.className = 'pill pill-run';
    if (run.date === currentRun) pill.classList.add('active');
    pill.dataset.run = run.date;
    pill.textContent = formatRunDate(run.date);
    pill.title = run.generated_at
      ? `${run.date} · ${run.total_count} questions · ${run.generated_at}`
      : run.date;
    pill.addEventListener('click', () => setRun(run.date));
    container.appendChild(pill);
  });
}

async function setRun(dateStr) {
  if (!dateStr || dateStr === currentRun) return;
  currentRun = dateStr;
  renderRunPicker();

  // Reload the manifest + categories from the new run, then reset
  // the active question card so the user picks fresh from this day.
  try {
    await loadManifest();
  } catch (e) {
    console.error('Failed to load manifest for run', dateStr, e);
    return;
  }
  questions = [];
  currentIndex = 0;
  selectedKey = null;
  revealed = false;

  renderLanding();
  updateProgress();
}

/* ---- Load questions ---- */

async function fetchCategoryQuestions(category) {
  // Category payloads are the bulky part of a run, so the worker writes
  // them gzip-compressed. Decode client-side via the native
  // DecompressionStream API and fall back to the plain .json file for
  // older runs that pre-date the compression switch.
  try {
    const gzRes = await fetch(quizPath(`${category}.json.gz`), { cache: 'no-store' });
    if (gzRes.ok) {
      const stream = gzRes.body.pipeThrough(new DecompressionStream('gzip'));
      const data = await new Response(stream).json();
      return data.questions || [];
    }
  } catch (e) {
    // Network error or no DecompressionStream support — try the plain
    // file before giving up.
    console.warn('Compressed category fetch failed, trying .json fallback', category, e);
  }
  const res = await fetch(quizPath(`${category}.json`), { cache: 'no-store' });
  const data = await res.json();
  return data.questions || [];
}

/* ---- Landing ---- */

function resumableSession() {
  const s = state.sessions && state.sessions[0];
  if (!s) return null;
  if (s.ended_at !== null) return null;
  if (s.abandoned) return null;
  if (!Array.isArray(s.queue) || s.queue.length === 0) return null;
  if (!Array.isArray(s.history) || s.history.length === 0) return null;
  // history.length === queue.length is ALLOWED: clicking Resume falls
  // through to renderRecap so the user still gets their results screen.
  if (s.history.length > s.queue.length) return null;
  return s;
}

// A session is "valid" if it has the minimum fields needed to render
// a usable row in the history drawer. Anything missing started_at or id
// is garbage (usually from an aborted startSession path or schema drift).
function isValidSession(s) {
  return s
    && typeof s.id === 'string'
    && Number.isFinite(s.started_at);
}

// On load, auto-complete any fully-answered session that never hit recap,
// and drop any malformed sessions that would render as "Invalid Date".
function sweepCompletedUnstamped() {
  if (!Array.isArray(state.sessions)) return;
  let dirty = false;

  const before = state.sessions.length;
  state.sessions = state.sessions.filter(isValidSession);
  if (state.sessions.length !== before) dirty = true;

  for (const s of state.sessions) {
    if (s.ended_at === null
        && Array.isArray(s.queue)
        && Array.isArray(s.history)
        && s.history.length > 0
        && s.history.length >= s.queue.length) {
      const last = s.history[s.history.length - 1];
      s.ended_at = last && last.timestamp ? last.timestamp : Date.now();
      dirty = true;
    }
  }
  if (dirty) saveState();
}

function fmtRelative(ms) {
  const diff = Date.now() - ms;
  if (diff < 0) return 'just now';
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return 'just now';
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}

function renderLanding() {
  mode = 'landing';
  stopMainTimer(true);
  hideQuizControls();
  const card = document.querySelector('.question-card');
  const totalCount = manifest?.categories?.reduce((n, c) => n + c.question_count, 0) ?? 0;
  const runLabel = currentRun ? formatRunDate(currentRun) : 'latest';

  const resumable = resumableSession();
  const resumeBannerHtml = resumable ? `
    <div class="resume-banner">
      <div class="resume-banner-title">Resume unfinished session?</div>
      <div class="resume-banner-meta">
        ${resumable.mode === 'custom' ? 'Custom' : 'Quick start'}
        &middot; ${resumable.history.length}/${resumable.planned}
        &middot; started ${escapeHtml(fmtRelative(resumable.started_at))}
      </div>
      <div class="resume-banner-actions">
        <button class="btn btn-primary" id="resumeSessionBtn">Resume</button>
        <button class="btn btn-ghost" id="abandonSessionBtn">Start fresh</button>
      </div>
    </div>
  ` : '';

  card.innerHTML = `
    <div class="landing">
      ${resumeBannerHtml}
      <div class="landing-title">Today's Quiz</div>
      <div class="landing-meta">${totalCount} questions · ${runLabel}</div>
      <div class="landing-actions">
        <button class="btn btn-primary landing-start" id="landingStartBtn">Quick start</button>
        <button class="btn btn-ghost landing-customize" id="landingCustomizeBtn">Pick Topics</button>
      </div>
      <div class="landing-hint">Press Enter to start</div>
    </div>
  `;

  document.querySelector('.answers').innerHTML = '';
  document.getElementById('explanation').style.display = 'none';
  const submitBtn = document.getElementById('submitBtn');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Submit';

  document.getElementById('landingStartBtn').addEventListener('click', startAutoSession);
  document.getElementById('landingCustomizeBtn').addEventListener('click', openCustomize);
  if (resumable) {
    document.getElementById('resumeSessionBtn').addEventListener('click', () => resumeSession());
    document.getElementById('abandonSessionBtn').addEventListener('click', abandonUnfinishedSession);
  }
}

function resumeSession(sess) {
  // Defensive: if called from an event listener, `sess` will be an Event.
  // Fall back to the banner-detected resumable session in that case.
  if (!sess || !Array.isArray(sess.queue) || !Array.isArray(sess.history)) {
    sess = resumableSession();
  }
  if (!sess) { renderLanding(); return; }
  // Make sure the session being resumed is at state.sessions[0] so
  // submitAnswer / nextQuestion target the right record.
  if (state.sessions && state.sessions[0] !== sess) {
    state.sessions = [sess, ...state.sessions.filter(s => s !== sess)];
    saveState();
  }
  sessionLength = sess.planned || 10;
  sessionAnswered = sess.total || sess.history.length || 0;
  sessionCorrect = sess.correct || 0;
  sessionWrongItems = Array.isArray(sess.wrongItems) ? sess.wrongItems.slice() : [];
  questions = sess.queue.slice();
  // Jump past already-answered questions
  currentIndex = Math.min(sess.history.length, questions.length);
  timerState.sessionStart = sess.started_at || Date.now();
  // Restore the current question's lap time. When the session was last saved
  // (tab close / visibilitychange / tick persist), we stashed how long the
  // user had already spent on this question; pick up where they left off
  // instead of resetting the lap to 0 on resume.
  const resumedLapMs = Number.isFinite(sess.currentQElapsedMs) ? sess.currentQElapsedMs : 0;
  timerState.qStart = Date.now() - Math.max(0, resumedLapMs);
  resumingSession = true;
  mode = 'quiz';

  if (currentIndex >= questions.length) {
    renderRecap();
    return;
  }

  startMainTimer();
  showQuestion();
}

function abandonUnfinishedSession() {
  const sess = state.sessions && state.sessions[0];
  if (sess && sess.ended_at === null) {
    sess.ended_at = Date.now();
    sess.abandoned = true;
    saveState();
  }
  renderLanding();
}

function showQuizControls() {
  const el = document.getElementById('quizControls');
  if (el) el.style.display = 'flex';
}

function hideQuizControls() {
  const el = document.getElementById('quizControls');
  if (el) el.style.display = 'none';
}

async function restartSession() {
  if (mode !== 'quiz') return;
  const ok = await showConfirm({
    title: 'Start over?',
    body: 'Restart this session from question 1. Your current answers will be kept as an abandoned attempt in the history drawer.',
    confirmLabel: 'Start over',
    cancelLabel: 'Cancel',
    danger: true
  });
  if (!ok) return;
  const sess = state.sessions && state.sessions[0];
  if (!sess) { renderLanding(); return; }

  // Close out the current attempt.
  const originalQueue = Array.isArray(sess.queue) ? sess.queue.slice() : questions.slice();
  sess.ended_at = Date.now();
  sess.abandoned = true;
  saveState();

  // Kick off a fresh run with the same queue + config.
  startSession(originalQueue, sess.planned || originalQueue.length, {
    mode: sess.mode || 'auto',
    config: sess.config || null
  });
}

function showSessionScope() {
  if (mode !== 'quiz') return;
  const sess = state.sessions && state.sessions[0];

  const fallbackCatList = (manifest?.categories ?? []).map(c => c.category);

  const modeLabel = sess?.mode === 'custom' ? 'Custom' : 'Quick start';

  const catIds = sess?.config?.categories?.length
    ? sess.config.categories
    : (sess?.mode === 'custom' ? fallbackCatList : fallbackCatList);
  const catsDisplay = (sess?.config?.categories?.length)
    ? sess.config.categories.map(formatCategoryLabel).join(', ')
    : `All categories (${fallbackCatList.length})`;

  const diffsDisplay = sess?.config?.difficulties?.length
    ? sess.config.difficulties.join(', ')
    : 'All difficulties';

  const lengthDisplay = Number.isFinite(sess?.planned)
    ? `${sess.planned} questions`
    : 'Unlimited';

  const progressDisplay = sess
    ? `${sess.total ?? 0} / ${sess?.planned ?? (Array.isArray(sess?.queue) ? sess.queue.length : '?')} answered`
    : 'no active session';

  const row = (label, value) => `
    <div class="scope-row">
      <div class="scope-label">${escapeHtml(label)}</div>
      <div class="scope-value">${escapeHtml(value)}</div>
    </div>`;

  const body = `
    <div class="scope-list">
      ${row('Mode', modeLabel)}
      ${row('Categories', catsDisplay)}
      ${row('Difficulty', diffsDisplay)}
      ${row('Length', lengthDisplay)}
      ${row('Progress', progressDisplay)}
    </div>
  `;

  showConfirm({
    title: 'Session status',
    body,
    html: true,
    info: true,
    cancelLabel: 'Close'
  });
}

async function quitSession() {
  if (mode !== 'quiz') return;
  const ok = await showConfirm({
    title: 'Quit this session?',
    body: 'End this session and return to the landing screen. Your answers stay in history as an abandoned attempt.',
    confirmLabel: 'Quit',
    cancelLabel: 'Keep going',
    danger: true
  });
  if (!ok) return;
  abandonUnfinishedSession();
}

function renderAbout() {
  mode = 'landing';
  hideQuizControls();
  const card = document.querySelector('.question-card');
  card.innerHTML = `
    <div class="about">
      <div class="about-header">
        <span class="about-title">About temporal.quiz</span>
        <button class="btn btn-ghost" id="aboutBackBtn">Back</button>
      </div>
      <p class="about-lead">A self-evaluation tool to gauge how well you actually know Temporal.</p>
      <p>Reading the docs feels like understanding them. Answering questions is how you find out. I built this so I (and anyone else who works with Temporal) can spot the gaps before they turn into 3am production pages.</p>
      <p>Questions are generated daily from the <a href="https://docs.temporal.io/" target="_blank" rel="noopener noreferrer">official Temporal docs</a>, spanning Workflows, Activities, Workers, Nexus, SDKs, Cloud, and self-hosting. Every run is archived so you can revisit earlier quizzes via the Run picker.</p>
      <p>Four difficulty tiers, from warmup to nightmare. Your streak and per-category accuracy live in your browser only. Nothing leaves your device.</p>
      <p class="about-muted">Not affiliated with Temporal Technologies. Built with Claude + a lot of curiosity.</p>
      <div class="about-actions">
        <button class="btn btn-primary" id="aboutStartBtn">Got it, let's quiz</button>
      </div>
    </div>
  `;
  document.querySelector('.answers').innerHTML = '';
  document.getElementById('explanation').style.display = 'none';
  const submitBtn = document.getElementById('submitBtn');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Submit';

  document.getElementById('aboutBackBtn').addEventListener('click', renderLanding);
  document.getElementById('aboutStartBtn').addEventListener('click', renderLanding);
}

function openCustomize() {
  mode = 'landing';
  hideQuizControls();
  const card = document.querySelector('.question-card');

  card.innerHTML = `
    <div class="customize">
      <div class="customize-header">
        <span class="customize-title">Build Your Session</span>
        <button class="btn btn-ghost" id="customizeBackBtn">Back</button>
      </div>

      <div class="customize-row">
        <div class="customize-label">Categories <span class="customize-hint">(empty = all)</span></div>
        <div class="customize-grid" id="customCats"></div>
      </div>

      <div class="customize-row">
        <div class="customize-label">Difficulty</div>
        <div class="customize-grid" id="customDiffs"></div>
      </div>

      <div class="customize-row">
        <div class="customize-label">Length</div>
        <div class="customize-grid" id="customLens"></div>
      </div>

      <div class="customize-actions">
        <button class="btn btn-primary" id="customizeStartBtn">Start</button>
      </div>
    </div>
  `;

  document.querySelector('.answers').innerHTML = '';
  document.getElementById('explanation').style.display = 'none';
  const submitBtn = document.getElementById('submitBtn');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Submit';

  renderCustomCategories();
  renderCustomDifficulties();
  renderCustomLengths();

  document.getElementById('customizeBackBtn').addEventListener('click', renderLanding);
  document.getElementById('customizeStartBtn').addEventListener('click', startCustomSession);
}

function toggleChip(arr, value) {
  const i = arr.indexOf(value);
  if (i >= 0) arr.splice(i, 1);
  else arr.push(value);
  saveState();
}

const CUSTOM_CATEGORY_GROUPS = [
  {
    label: 'Core Features',
    categories: [
      { id: 'Features_Workflows',               label: 'Workflows' },
      { id: 'Features_Activities',              label: 'Activities' },
      { id: 'Features_Workers',                 label: 'Workers' },
      { id: 'Features_TaskQueues',              label: 'Task Queues' },
      { id: 'Features_Signals_Queries_Updates', label: 'Signals / Queries / Updates' },
      { id: 'Features_Schedules',               label: 'Schedules' },
      { id: 'Features_Timers_Retries',          label: 'Timers & Retries' },
      { id: 'Features_Nexus',                   label: 'Nexus' },
    ],
  },
  {
    label: 'Execution & State',
    categories: [
      { id: 'Features_History',          label: 'History & Replay' },
      { id: 'Features_WorkerVersioning', label: 'Worker Versioning' },
      { id: 'Features_Composition',      label: 'Composition' },
      { id: 'Features_Visibility',       label: 'Visibility' },
      { id: 'Features_Export',           label: 'Export' },
    ],
  },
  {
    label: 'Data & Security',
    categories: [
      { id: 'Features_DataConversion',    label: 'Data Conversion' },
      { id: 'Features_Security',          label: 'Security' },
      { id: 'Features_ConnectivityRules', label: 'Connectivity Rules' },
    ],
  },
  {
    label: 'Operations',
    categories: [
      { id: 'Features_Namespaces',       label: 'Namespaces' },
      { id: 'Features_Observability',    label: 'Metrics & Monitoring' },
      { id: 'Features_HighAvailability', label: 'HA & Disaster Recovery' },
      { id: 'Features_AuditLogging',     label: 'Audit Logging' },
      { id: 'Operations_TemporalCloud',  label: 'Temporal Cloud' },
      { id: 'Operations_SelfHosted',     label: 'Self Hosted' },
    ],
  },
  {
    label: 'SDKs',
    categories: [
      { id: 'Develop', label: 'Develop (General)' },
    ],
  },
  {
    label: 'Tooling',
    categories: [
      { id: 'Tooling_CLI',         label: 'CLI' },
      { id: 'Tooling_WebUI_API',   label: 'Web UI & API' },
      { id: 'Tooling_AI_Cookbook', label: 'AI & Cookbook' },
    ],
  },
  {
    label: 'Concepts',
    categories: [
      { id: 'General_Concepts', label: 'General Concepts' },
    ],
  },
  {
    label: 'Other',
    categories: [
      { id: 'Miscellaneous', label: 'Miscellaneous' },
    ],
  },
];

function renderCustomCategories() {
  const host = document.getElementById('customCats');
  if (!host) return;
  host.innerHTML = '';

  const manifestIds = new Set((manifest?.categories ?? []).map(c => c.category));
  const renderedIds = new Set();
  const selected = new Set(customConfig.categories);

  const makePill = (id, label) => {
    const pill = document.createElement('div');
    pill.className = 'pill';
    if (selected.has(id)) pill.classList.add('active');
    pill.textContent = label;
    pill.title = formatCategoryLabel(id);
    pill.addEventListener('click', () => {
      toggleChip(customConfig.categories, id);
      renderCustomCategories();
    });
    return pill;
  };

  const makeAllPill = (label, ids) => {
    const pill = document.createElement('div');
    pill.className = 'pill pill-all-toggle';
    const allSelected = ids.length > 0 && ids.every(id => selected.has(id));
    if (allSelected) pill.classList.add('active');
    pill.textContent = label;
    pill.title = allSelected ? `Deselect ${label.toLowerCase()}` : `Select ${label.toLowerCase()}`;
    pill.addEventListener('click', () => {
      if (allSelected) {
        customConfig.categories = customConfig.categories.filter(id => !ids.includes(id));
      } else {
        // Merge without duplicates.
        customConfig.categories = Array.from(new Set([...customConfig.categories, ...ids]));
      }
      saveState();
      renderCustomCategories();
    });
    return pill;
  };

  // Top-level All row (every manifest category)
  const allIds = [...manifestIds];
  const topRow = document.createElement('div');
  topRow.className = 'customize-group customize-group-top';
  const topRowInner = document.createElement('div');
  topRowInner.className = 'customize-group-row';
  topRowInner.appendChild(makeAllPill('All', allIds));
  topRow.appendChild(topRowInner);
  host.appendChild(topRow);

  // Per-group rows with a group-level All
  CUSTOM_CATEGORY_GROUPS.forEach(group => {
    const present = group.categories.filter(c => manifestIds.has(c.id));
    if (present.length === 0) return;

    const groupEl = document.createElement('div');
    groupEl.className = 'customize-group';

    const labelEl = document.createElement('div');
    labelEl.className = 'customize-group-label';
    labelEl.textContent = group.label;
    groupEl.appendChild(labelEl);

    const row = document.createElement('div');
    row.className = 'customize-group-row';
    row.appendChild(makeAllPill('All', present.map(c => c.id)));
    present.forEach(c => {
      row.appendChild(makePill(c.id, c.label));
      renderedIds.add(c.id);
    });
    groupEl.appendChild(row);
    host.appendChild(groupEl);
  });

  // Catch-all: any manifest category not in any known group
  const ungrouped = [...manifestIds].filter(id => !renderedIds.has(id)).sort();
  if (ungrouped.length > 0) {
    const groupEl = document.createElement('div');
    groupEl.className = 'customize-group';

    const labelEl = document.createElement('div');
    labelEl.className = 'customize-group-label';
    labelEl.textContent = 'More';
    groupEl.appendChild(labelEl);

    const row = document.createElement('div');
    row.className = 'customize-group-row';
    row.appendChild(makeAllPill('All', ungrouped));
    ungrouped.forEach(id => row.appendChild(makePill(id, formatCategoryLabel(id))));
    groupEl.appendChild(row);
    host.appendChild(groupEl);
  }
}

function renderCustomDifficulties() {
  const host = document.getElementById('customDiffs');
  if (!host) return;
  host.innerHTML = '';

  // "All" chip — active when no specific difficulty is picked (an empty
  // filter already means every difficulty is in). Clicking it resets the
  // selection. Clicking any specific chip deactivates All implicitly.
  const allChip = document.createElement('div');
  allChip.className = 'diff-chip diff-chip-all';
  if (customConfig.difficulties.length === 0) allChip.classList.add('active');
  allChip.innerHTML = '<span class="difficulty-badge difficulty-all"><span class="difficulty-label">All</span></span>';
  allChip.addEventListener('click', () => {
    customConfig.difficulties = [];
    renderCustomDifficulties();
  });
  host.appendChild(allChip);

  const diffs = ['easy', 'med', 'hard', 'nightmare'];
  // When All is active (no explicit picks), show every specific chip in
  // its active state too, so the "all are included" meaning is visually
  // unambiguous instead of the chips going dim.
  const allActive = customConfig.difficulties.length === 0;
  diffs.forEach(d => {
    const pill = document.createElement('div');
    pill.className = 'diff-chip';
    if (allActive || customConfig.difficulties.includes(d)) pill.classList.add('active');
    pill.innerHTML = difficultyDots(d);
    pill.addEventListener('click', () => {
      // First click out of the All state seeds the selection with ONLY
      // the clicked difficulty. Without this, the dim-all → dim-one
      // transition felt like a deselect rather than a pick.
      if (allActive) {
        customConfig.difficulties = [d];
      } else {
        toggleChip(customConfig.difficulties, d);
      }
      // Re-render so the All chip activates/deactivates in sync with the
      // specific picks.
      renderCustomDifficulties();
    });
    host.appendChild(pill);
  });
}

function renderCustomLengths() {
  const host = document.getElementById('customLens');
  if (!host) return;
  host.innerHTML = '';
  const options = [
    { value: 5,        label: '5'  },
    { value: 10,       label: '10' },
    { value: 20,       label: '20' },
    { value: Infinity, label: '∞'  },
  ];
  options.forEach(opt => {
    const pill = document.createElement('div');
    pill.className = 'pill';
    if (customConfig.length === opt.value) pill.classList.add('active');
    pill.textContent = opt.label;
    pill.addEventListener('click', () => {
      customConfig.length = opt.value;
      saveState();
      renderCustomLengths();
    });
    host.appendChild(pill);
  });
}

async function startCustomSession() {
  const chosenCats = customConfig.categories.length > 0
    ? customConfig.categories
    : (manifest?.categories?.map(c => c.category) ?? []);

  const settled = await Promise.all(chosenCats.map(async cat => {
    try {
      return await fetchCategoryQuestions(cat);
    } catch (e) {
      console.error('Failed to load category for custom session', cat, e);
      return [];
    }
  }));
  let pool = settled.flat();

  if (customConfig.difficulties.length > 0) {
    pool = pool.filter(q => customConfig.difficulties.includes(q.difficulty));
  }

  if (pool.length === 0) {
    openCustomize();
    const host = document.querySelector('.customize');
    if (host) {
      const msg = document.createElement('div');
      msg.className = 'customize-empty';
      msg.textContent = 'No questions match this filter. Widen selection.';
      host.appendChild(msg);
    }
    return;
  }

  startSession(shuffle(pool), customConfig.length, {
    mode: 'custom',
    config: {
      categories: [...customConfig.categories],
      difficulties: [...customConfig.difficulties],
      length: customConfig.length
    }
  });
}

/* ---- Session ---- */

async function startAutoSession() {
  const fetched = await Promise.all(manifest.categories.map(async cat => {
    try {
      const qs = await fetchCategoryQuestions(cat.category);
      return { category: cat.category, qs };
    } catch (e) {
      console.error('Failed to load category for auto session', cat.category, e);
      return { category: cat.category, qs: [] };
    }
  }));
  const all = [];
  for (const { category, qs } of fetched) {
    const stats = state.categoryStats[category];
    const accuracy = stats && stats.answered > 0 ? stats.correct / stats.answered : 1;
    if (accuracy < 0.7) all.push(...qs, ...qs);
    else all.push(...qs);
  }
  startSession(shuffle(all), sessionLength, { mode: 'auto' });
}

function startSession(qs, length, meta = {}) {
  sessionLength = length;
  sessionAnswered = 0;
  sessionCorrect = 0;
  sessionWrongItems = [];
  timerState.sessionStart = Date.now();
  questions = Number.isFinite(length) ? qs.slice(0, length) : qs;
  currentIndex = 0;
  mode = 'quiz';
  if (questions.length === 0) {
    renderLanding();
    return;
  }

  // Close out any prior unfinished session so it shows up correctly in history.
  if (!state.sessions) state.sessions = [];
  const prev = state.sessions[0];
  if (prev && prev.ended_at === null) {
    prev.ended_at = Date.now();
    if (prev.history && prev.history.length < (prev.planned || prev.queue?.length || 0)) {
      prev.abandoned = true;
    }
  }

  const sess = {
    id: String(Date.now()) + '-' + Math.random().toString(36).slice(2, 6),
    started_at: Date.now(),
    ended_at: null,
    mode: meta.mode || 'auto',
    config: meta.config || null,
    planned: questions.length,
    total: 0,
    correct: 0,
    history: [],
    queue: questions.slice(),     // full question objects for resume
    currentIndex: 0,
    wrongItems: [],
    abandoned: false
  };
  state.sessions.unshift(sess);
  if (state.sessions.length > 50) state.sessions.length = 50;
  saveState();

  showQuestion();
}

/* ---- Display question ---- */

// isLastQuestion reports whether the current question is the final one in a
// finite session. Infinite sessions reshuffle on overflow so there's no
// "last" to label. Used to swap Submit/Next -> Finish at the right moment.
function isLastQuestion() {
  return Number.isFinite(sessionLength) && currentIndex === questions.length - 1;
}

function showQuestion() {
  if (!questions.length) return;
  if (mode !== 'quiz') mode = 'quiz';
  // resumeSession() pre-seeds qStart with the saved lap offset before
  // calling showQuestion() so the lap picks up where it left off. For
  // every other call path (normal flow, next question, restart), start
  // a fresh lap and clear any stale persisted elapsed.
  if (!resumingSession) {
    timerState.qStart = Date.now();
    resetCurrentLap();
  }
  resumingSession = false;
  startMainTimer();
  showQuizControls();
  scratchText = '';
  if (state.toolbox?.open && state.toolbox?.tab === 'scratch') {
    renderToolboxBody('scratch');
  }

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

  // Reset submit button. On the last question of a finite session, use
  // "Finish" so the user sees that submitting this one wraps up the run.
  const btn = document.getElementById('submitBtn');
  btn.textContent = isLastQuestion() ? 'Finish' : 'Submit';
  btn.disabled = true;

  // Show Skip for a fresh question
  const skip = document.getElementById('skipBtn');
  if (skip) skip.style.display = '';

  updatePrevButton();
  updateProgress();

  // If this index already has a recorded answer, replay it in read-only mode.
  const activeSess = state.sessions && state.sessions[0];
  const historyEntry = activeSess?.history?.[currentIndex];
  if (historyEntry) {
    replayAnsweredQuestion(q, historyEntry);
  }
}

function replayAnsweredQuestion(q, h) {
  // Set internal state so keyboard shortcuts behave as "answered".
  revealed = true;
  selectedKey = h.selectedKey;

  // Apply correct/wrong/dimmed styling without re-running stats.
  document.querySelectorAll('.answer').forEach(a => {
    const k = a.querySelector('.answer-key').textContent.trim();
    a.classList.remove('selected');
    if (k === q.answer) {
      a.classList.add('correct');
    } else if (k === h.selectedKey && k !== q.answer) {
      a.classList.add('wrong');
    } else {
      a.classList.add('dimmed');
    }
  });

  // Re-show the explanation.
  const explanation = document.getElementById('explanation');
  explanation.style.display = 'block';
  explanation.innerHTML = `
    <div class="explanation-label">Explanation</div>
    <div class="explanation-text">${formatQuestion(q.explanation)}</div>
  `;

  // Submit becomes Next (or Finish on the last question) so the user
  // can move forward again.
  const btn = document.getElementById('submitBtn');
  btn.textContent = isLastQuestion() ? 'Finish' : 'Next';
  btn.disabled = false;

  // No Skip on an already-answered question.
  const skip = document.getElementById('skipBtn');
  if (skip) skip.style.display = 'none';
}

function updatePrevButton() {
  const prev = document.getElementById('prevBtn');
  if (!prev) return;
  prev.disabled = !(mode === 'quiz' && currentIndex > 0);
}

function goPrevQuestion() {
  if (mode !== 'quiz' || currentIndex <= 0) return;
  currentIndex--;
  showQuestion();
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

  sessionAnswered++;
  if (isCorrect) {
    sessionCorrect++;
  } else {
    sessionWrongItems.push({
      question: q.question,
      choices: q.choices,
      answer: q.answer,
      selectedKey,
      difficulty: q.difficulty,
      category: q.category,
      source_doc: q.source_doc || ''
    });
  }

  // Category stats
  if (!state.categoryStats[q.category]) {
    state.categoryStats[q.category] = { answered: 0, correct: 0 };
  }
  state.categoryStats[q.category].answered++;
  if (isCorrect) {
    state.categoryStats[q.category].correct++;
  }

  // Session history (append to the active session record)
  const activeSess = state.sessions && state.sessions[0];
  if (activeSess) {
    activeSess.history.push({
      question: q.question.substring(0, 150),
      category: q.category,
      subcategory: formatSubcategory(q.source_doc),
      difficulty: q.difficulty,
      correct: isCorrect,
      selectedKey,
      answer: q.answer,
      timestamp: Date.now()
    });
    activeSess.total++;
    if (isCorrect) activeSess.correct++;
    activeSess.wrongItems = sessionWrongItems.slice();
    activeSess.currentIndex = currentIndex;
  }

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

  // Inline per-question note block (Phase 3)
  const qnKey = hashQuestion(q.question);
  const qnExisting = state.questionNotes[qnKey];

  const qnBlock = document.createElement('div');
  qnBlock.className = 'qn-block';
  qnBlock.innerHTML = `
    <button class="btn btn-ghost qn-toggle" id="qnToggleBtn">
      ${qnExisting ? '&#128221; Edit note' : '&#128221; Add note'}
    </button>
    <div class="qn-editor" id="qnEditor" style="display:none;">
      <textarea class="notes-textarea qn-text" id="qnText" rows="4"
                placeholder="What made this tricky? What to remember next time?"></textarea>
      <div class="qn-actions">
        <button class="btn btn-ghost qn-delete" id="qnDeleteBtn">Delete</button>
      </div>
    </div>
  `;
  explanation.appendChild(qnBlock);

  const qnEditor = document.getElementById('qnEditor');
  const qnTa = document.getElementById('qnText');
  const qnToggle = document.getElementById('qnToggleBtn');
  const qnDel = document.getElementById('qnDeleteBtn');

  qnTa.value = qnExisting?.note ?? '';
  qnToggle.addEventListener('click', () => {
    qnEditor.style.display = qnEditor.style.display === 'none' ? 'block' : 'none';
    if (qnEditor.style.display !== 'none') qnTa.focus();
  });

  qnTa.addEventListener('input', debounce(() => {
    const text = qnTa.value;
    if (!text.trim()) {
      delete state.questionNotes[qnKey];
    } else {
      const now = Date.now();
      const prev = state.questionNotes[qnKey];
      state.questionNotes[qnKey] = {
        hash: qnKey,
        note: text,
        question: q.question,
        source_doc: q.source_doc || '',
        category: q.category || '',
        created_at: prev?.created_at ?? now,
        updated_at: now
      };
    }
    saveState();
    qnToggle.innerHTML = state.questionNotes[qnKey] ? '&#128221; Edit note' : '&#128221; Add note';
  }, 300));

  qnDel.addEventListener('click', () => {
    delete state.questionNotes[qnKey];
    qnTa.value = '';
    saveState();
    qnToggle.innerHTML = '&#128221; Add note';
    qnEditor.style.display = 'none';
  });

  // Switch button to Next (or Finish on the last question).
  const btn = document.getElementById('submitBtn');
  btn.textContent = isLastQuestion() ? 'Finish' : 'Next';
  btn.disabled = false;
}

/* ---- Next / Skip ---- */

function nextQuestion() {
  currentIndex++;
  const activeSess = state.sessions && state.sessions[0];
  if (activeSess && activeSess.ended_at === null) {
    activeSess.currentIndex = currentIndex;
    saveState();
  }
  if (currentIndex >= questions.length) {
    if (Number.isFinite(sessionLength)) {
      renderRecap();
      return;
    }
    // Infinite session: reshuffle and restart
    questions = shuffle(questions);
    currentIndex = 0;
    if (activeSess && activeSess.ended_at === null) {
      activeSess.queue = questions.slice();
      activeSess.currentIndex = 0;
      saveState();
    }
  }
  showQuestion();
}

// renderRecap can render the live session (no arg — reads the active
// session at state.sessions[0] and the module-level counters) or a
// specific session replayed from history. In the latter case we skip
// the end-time stamping and confetti so the history viewer doesn't
// mutate completed records or celebrate past runs on every open.
function renderRecap(sess) {
  const fromHistory = !!sess;
  mode = 'recap';
  timerState.qStart = null;
  stopMainTimer();
  hideQuizControls();
  const activeSess = sess || (state.sessions && state.sessions[0]);
  // Stamp end time only for a LIVE in-progress session finishing up.
  if (!fromHistory && activeSess && activeSess.ended_at === null) {
    activeSess.ended_at = Date.now();
    saveState();
  }
  const card = document.querySelector('.question-card');
  // When replaying from history, pull totals from the stored session so
  // the recap reflects that session, not whatever module-level state the
  // live quiz left behind.
  const total = fromHistory
    ? (Number.isFinite(activeSess?.total) ? activeSess.total : (activeSess?.history?.length ?? 0))
    : sessionAnswered;
  const correct = fromHistory ? (activeSess?.correct ?? 0) : sessionCorrect;
  const wrongItems = fromHistory
    ? (Array.isArray(activeSess?.wrongItems) ? activeSess.wrongItems : [])
    : sessionWrongItems;
  const wrong = Math.max(0, total - correct);
  const pct = total > 0 ? Math.round((correct / total) * 100) : 0;
  const durationMs = (activeSess && activeSess.started_at && activeSess.ended_at)
    ? activeSess.ended_at - activeSess.started_at
    : 0;
  const durationStr = durationMs > 0 ? fmtSessionDuration(durationMs) : '';

  // Donut chart built from explicit SVG arc paths. This replaces the
  // old stroke-dasharray approach which silently produced empty arcs on
  // some WebKit builds (CSS-declared stroke URLs collapsed to stroke:none
  // when the SVG was inserted via innerHTML). Arc paths are pure geometry
  // and cannot fail at render time.
  const R = 60;
  const CX = 90;
  const CY = 90;
  const pctTone = pct >= 80 ? 'good' : pct >= 50 ? 'ok' : 'poor';

  const polar = (angleDeg) => {
    const a = (angleDeg - 90) * Math.PI / 180; // rotate so 0deg is 12 o'clock
    return [CX + R * Math.cos(a), CY + R * Math.sin(a)];
  };
  const arcPath = (startDeg, endDeg) => {
    // Near-full-circle: close on itself to avoid the A-command edge case
    // where start == end draws nothing.
    if (endDeg - startDeg >= 359.99) {
      const [sx, sy] = polar(0);
      const [ex, ey] = polar(180);
      return `M ${sx} ${sy} A ${R} ${R} 0 1 1 ${ex} ${ey} A ${R} ${R} 0 1 1 ${sx} ${sy}`;
    }
    const [sx, sy] = polar(startDeg);
    const [ex, ey] = polar(endDeg);
    const largeArc = (endDeg - startDeg) > 180 ? 1 : 0;
    return `M ${sx} ${sy} A ${R} ${R} 0 ${largeArc} 1 ${ex} ${ey}`;
  };

  const correctDeg = total > 0 ? (correct / total) * 360 : 0;
  const correctArc = correct > 0 ? `<path d="${arcPath(0, correctDeg)}" stroke="#22c55e" fill="none" stroke-width="14" stroke-linecap="round"/>` : '';
  const wrongArc   = wrong   > 0 ? `<path d="${arcPath(correctDeg, 360)}" stroke="#ef4444" fill="none" stroke-width="14" stroke-linecap="round"/>` : '';

  const wrongListHtml = wrongItems.length === 0 ? '' : `
    <div class="recap-wrong-list">
      <div class="recap-wrong-label">Wrong answers (${wrongItems.length})</div>
      ${wrongItems.map(w => {
        const userChoice = (w.choices.find(c => c.key === w.selectedKey) || {}).text || '';
        const correctChoice = (w.choices.find(c => c.key === w.answer) || {}).text || '';
        return `
          <div class="recap-wrong-item">
            <div class="recap-wrong-meta">
              <span>${escapeHtml(formatCategoryLabel(w.category || ''))}</span>
              <span>${difficultyDots(w.difficulty)}</span>
            </div>
            <div class="recap-wrong-q">${formatQuestion(w.question)}</div>
            <div class="recap-wrong-answers">
              <div class="recap-wrong-row wrong">
                <span class="recap-key">${escapeHtml(w.selectedKey)}</span>
                <span class="recap-tag">your answer</span>
                <span class="recap-text">${formatQuestion(userChoice)}</span>
              </div>
              <div class="recap-wrong-row correct">
                <span class="recap-key">${escapeHtml(w.answer)}</span>
                <span class="recap-tag">correct</span>
                <span class="recap-text">${formatQuestion(correctChoice)}</span>
              </div>
            </div>
          </div>
        `;
      }).join('')}
    </div>
  `;

  card.innerHTML = `
    <div class="landing recap">
      <div class="landing-title">Session complete</div>
      ${durationStr ? `<div class="recap-took">Took ${escapeHtml(durationStr)}</div>` : ''}

      <div class="recap-chart">
        <svg class="recap-donut" width="180" height="180" viewBox="0 0 180 180">
          <circle cx="${CX}" cy="${CY}" r="${R}" class="recap-donut-bg"></circle>
          ${correctArc}
          ${wrongArc}
          <text x="${CX}" y="${CY + 2}" class="recap-donut-pct recap-pct-${pctTone}" text-anchor="middle" dominant-baseline="middle">${pct}%</text>
          <text x="${CX}" y="${CY + 28}" class="recap-donut-sub" text-anchor="middle">${correct} / ${total}</text>
        </svg>
        <div class="recap-legend">
          <div class="recap-legend-chip correct">
            <span class="recap-legend-dot"></span>
            <span class="recap-legend-label">Correct</span>
            <span class="recap-legend-value">${correct}</span>
          </div>
          <div class="recap-legend-chip wrong">
            <span class="recap-legend-dot"></span>
            <span class="recap-legend-label">Wrong</span>
            <span class="recap-legend-value">${wrong}</span>
          </div>
        </div>
      </div>

      ${wrongListHtml}

      <div class="landing-actions">
        <button class="btn btn-primary" id="recapAgainBtn">Start again</button>
      </div>
    </div>
  `;
  document.querySelector('.answers').innerHTML = '';
  const explanation = document.getElementById('explanation');
  explanation.style.display = 'none';
  explanation.innerHTML = '';
  const submitBtn = document.getElementById('submitBtn');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Submit';
  document.getElementById('recapAgainBtn').addEventListener('click', renderLanding);

  // Celebrate finishing, period. Confetti fires regardless of score
  // because completing a session is worth acknowledging; we just dial
  // the burst size up for higher accuracy. Skipped when replaying a
  // past session from history — re-opening an old result shouldn't
  // re-fire the celebration every time.
  if (!fromHistory) fireConfetti(pct);
}

/* ---- Confetti ---- */

// Minimal canvas-based confetti burst. Spawns N particles from two
// origin points along the top edge, applies gravity + drag + rotation,
// and bows out within a few seconds. No deps. Respects prefers-reduced-
// motion so accessibility toggles actually turn it off.
function fireConfetti(pct) {
  const canvas = document.getElementById('confettiCanvas');
  if (!canvas) return;
  if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

  const dpr = Math.max(1, window.devicePixelRatio || 1);
  const w = window.innerWidth;
  const h = window.innerHeight;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  canvas.style.width = w + 'px';
  canvas.style.height = h + 'px';
  canvas.classList.add('firing');
  const ctx = canvas.getContext('2d');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  const colors = ['#7c3aed', '#a855f7', '#22c55e', '#f59e0b', '#ef4444', '#38bdf8', '#ec4899'];
  // Scale the burst with accuracy but keep a floor so any completion
  // gets a real celebration.
  const count = Math.round(160 + (pct / 100) * 140);
  const particles = [];
  // Two cannons at the bottom corners, firing up and inward so the
  // confetti arcs toward the middle before gravity pulls it back down.
  const origins = [
    { x: w * 0.2, y: h + 20, dir:  1 }, // bottom-left -> up-right
    { x: w * 0.8, y: h + 20, dir: -1 }, // bottom-right -> up-left
  ];
  for (let i = 0; i < count; i++) {
    const o = origins[i % origins.length];
    // Launch angle measured from horizontal, 45-90 deg, toward center.
    const launchDeg = 45 + Math.random() * 45;
    const rad = launchDeg * Math.PI / 180;
    const speed = 18 + Math.random() * 14; // 18-32, enough to clear most viewports
    particles.push({
      x: o.x + (Math.random() - 0.5) * 60,
      y: o.y,
      vx: Math.cos(rad) * speed * o.dir,
      vy: -Math.sin(rad) * speed, // negative = upward
      size: 6 + Math.random() * 6,
      rot: Math.random() * Math.PI * 2,
      vr: (Math.random() - 0.5) * 0.35,
      color: colors[(Math.random() * colors.length) | 0],
      life: 1,
      // Slower decay so the burst lasts ~4-5s instead of ~2s.
      decay: 0.003 + Math.random() * 0.003,
    });
  }

  let raf = 0;
  const tick = () => {
    ctx.clearRect(0, 0, w, h);
    let alive = 0;
    for (const p of particles) {
      if (p.life <= 0) continue;
      p.vy += 0.18;        // gravity (slightly softer for longer hang time)
      p.vx *= 0.993;       // air drag
      p.vy *= 0.997;
      p.x += p.vx;
      p.y += p.vy;
      p.rot += p.vr;
      p.life -= p.decay;
      // Kill once the flake has clearly fallen past the bottom edge on
      // its way back down.
      if (p.vy > 0 && p.y > h + 40) { p.life = 0; continue; }
      alive++;

      ctx.save();
      ctx.translate(p.x, p.y);
      ctx.rotate(p.rot);
      ctx.globalAlpha = Math.max(0, p.life);
      ctx.fillStyle = p.color;
      // Simple rectangular flake — cheap and reads well on all backgrounds.
      ctx.fillRect(-p.size / 2, -p.size / 3, p.size, p.size / 1.6);
      ctx.restore();
    }

    if (alive > 0) {
      raf = requestAnimationFrame(tick);
    } else {
      canvas.classList.remove('firing');
      ctx.clearRect(0, 0, w, h);
    }
  };
  raf = requestAnimationFrame(tick);
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

  if ((e.key === 'n' || e.key === 'N') && (mode === 'landing' || mode === 'recap' || revealed)) {
    toggleNotes();
    e.preventDefault();
    return;
  }

  if (mode === 'landing' && e.key === 'Enter') {
    const startBtn = document.getElementById('landingStartBtn');
    if (startBtn) {
      startBtn.click();
      e.preventDefault();
      return;
    }
  }
  if (mode === 'recap' && e.key === 'Enter') {
    const againBtn = document.getElementById('recapAgainBtn');
    if (againBtn) {
      againBtn.click();
      e.preventDefault();
      return;
    }
  }

  if (mode === 'quiz' && e.key === 'ArrowLeft' && currentIndex > 0) {
    goPrevQuestion();
    e.preventDefault();
    return;
  }

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

function toggleNotes() {
  const open = document.getElementById('notesSidebar').classList.toggle('open');
  document.getElementById('notesSidebarOverlay').classList.toggle('open');
  if (open) renderNotes();
}

function renderNotes() {
  const list = document.getElementById('notesList');
  const cats = manifest?.categories?.map(c => c.category).sort() ?? [];

  if (cats.length === 0) {
    list.innerHTML = '<p class="history-empty">Load some quiz data first.</p>';
    return;
  }

  if (!notesActiveCategory || !cats.includes(notesActiveCategory)) {
    notesActiveCategory = cats[0];
  }

  list.innerHTML = `
    <div class="notes-pane">
      <div class="notes-section">
        <div class="notes-section-header">
          <label for="notesCategorySelect" class="notes-label">Category notes</label>
          <select id="notesCategorySelect" class="notes-select"></select>
        </div>
        <textarea id="notesCategoryText"
                  class="notes-textarea"
                  placeholder="Cheat sheet for this category. Markdown is fine."
                  rows="10"></textarea>
      </div>

      <div class="notes-section">
        <div class="notes-section-header">
          <span class="notes-label">Question notes</span>
          <span class="notes-count" id="notesQuestionCount"></span>
        </div>
        <div id="notesQuestionList" class="notes-question-list"></div>
      </div>
    </div>
  `;

  const select = document.getElementById('notesCategorySelect');
  cats.forEach(cat => {
    const opt = document.createElement('option');
    opt.value = cat;
    opt.textContent = formatCategoryLabel(cat);
    if (cat === notesActiveCategory) opt.selected = true;
    select.appendChild(opt);
  });
  select.addEventListener('change', () => {
    notesActiveCategory = select.value;
    syncCategoryTextarea();
  });

  syncCategoryTextarea();
  renderQuestionNotes();
}

function syncCategoryTextarea() {
  const ta = document.getElementById('notesCategoryText');
  if (!ta) return;
  ta.value = state.categoryNotes[notesActiveCategory] ?? '';
  ta.oninput = debounce(() => {
    state.categoryNotes[notesActiveCategory] = ta.value;
    saveState();
  }, 300);
}

function renderQuestionNotes() {
  const host = document.getElementById('notesQuestionList');
  const countEl = document.getElementById('notesQuestionCount');
  if (!host) return;

  const notes = Object.values(state.questionNotes || {})
    .sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0));

  if (countEl) countEl.textContent = `${notes.length} saved`;

  if (notes.length === 0) {
    host.innerHTML = '<p class="history-empty qn-empty">No question notes yet. Answer a question and click the note button.</p>';
    return;
  }

  host.innerHTML = notes.map(n => {
    const safeCat = escapeHtml(formatCategoryLabel(n.category || ''));
    const safeQ = escapeHtml(n.question);
    const safeNote = escapeHtml(n.note);
    return `
      <div class="qn-item" data-hash="${n.hash}">
        <div class="qn-item-meta">
          <span>${safeCat}</span>
          <button class="qn-item-del" data-hash="${n.hash}" title="Delete">&times;</button>
        </div>
        <div class="qn-item-question">${safeQ}</div>
        <div class="qn-item-note">${safeNote}</div>
      </div>
    `;
  }).join('');

  host.querySelectorAll('.qn-item-del').forEach(btn => {
    btn.addEventListener('click', () => {
      const h = btn.dataset.hash;
      delete state.questionNotes[h];
      saveState();
      renderQuestionNotes();
    });
  });
}

function exportNotes() {
  const cats = Object.keys(state.categoryNotes || {})
    .filter(k => (state.categoryNotes[k] || '').trim())
    .sort();
  const qNotes = Object.values(state.questionNotes || {})
    .sort((a, b) => (a.category || '').localeCompare(b.category || ''));

  if (cats.length === 0 && qNotes.length === 0) {
    alert('No notes to export yet.');
    return;
  }

  const lines = [`# temporal.quiz notes`, `Exported ${new Date().toISOString()}`, ''];

  if (cats.length > 0) {
    lines.push('## Category notes', '');
    for (const cat of cats) {
      lines.push(`### ${formatCategoryLabel(cat)}`, '', state.categoryNotes[cat].trim(), '');
    }
  }

  if (qNotes.length > 0) {
    lines.push('## Question notes', '');
    let lastCat = null;
    for (const n of qNotes) {
      if (n.category !== lastCat) {
        lines.push(`### ${formatCategoryLabel(n.category || 'Uncategorized')}`, '');
        lastCat = n.category;
      }
      lines.push(`**Q:** ${n.question}`);
      if (n.source_doc) lines.push(`_source: ${n.source_doc}_`);
      lines.push('', n.note.trim(), '');
    }
  }

  const blob = new Blob([lines.join('\n')], { type: 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `temporal-quiz-notes-${new Date().toISOString().slice(0,10)}.md`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function fmtSessionDateTime(ms) {
  const d = new Date(ms);
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' +
         d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

function fmtSessionDuration(ms) {
  if (!ms || ms <= 0) return '0s';
  const totalSec = Math.round(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;

  const totalMin = Math.floor(totalSec / 60);
  const secs = totalSec % 60;
  const pad = (n) => (n < 10 ? '0' + n : '' + n);

  if (totalMin < 60) {
    return `${totalMin}m ${pad(secs)}s`;
  }
  const hours = Math.floor(totalMin / 60);
  const mins = totalMin % 60;
  return `${hours}h ${pad(mins)}m ${pad(secs)}s`;
}

/* ---- Modal (custom confirm) ---- */

let modalResolver = null;
let modalKeyHandler = null;

function showConfirm({ title = 'Are you sure?', body = '', html = false, confirmLabel = 'Confirm', cancelLabel = 'Cancel', danger = false, info = false } = {}) {
  return new Promise(resolve => {
    const backdrop = document.getElementById('modalBackdrop');
    const titleEl = document.getElementById('modalTitle');
    const bodyEl = document.getElementById('modalBody');
    const confirmBtn = document.getElementById('modalConfirmBtn');
    const cancelBtn = document.getElementById('modalCancelBtn');
    if (!backdrop) { resolve(false); return; }

    titleEl.textContent = title;
    if (html) bodyEl.innerHTML = body; else bodyEl.textContent = body;
    confirmBtn.textContent = confirmLabel;
    cancelBtn.textContent = info ? (cancelLabel === 'Cancel' ? 'Close' : cancelLabel) : cancelLabel;
    confirmBtn.classList.toggle('btn-danger', !!danger);
    confirmBtn.style.display = info ? 'none' : '';

    backdrop.style.display = 'flex';
    modalResolver = resolve;

    // Focus the safe/close action by default on destructive or info prompts.
    setTimeout(() => ((danger || info) ? cancelBtn : confirmBtn).focus(), 0);

    modalKeyHandler = (e) => {
      if (e.key === 'Escape') { e.preventDefault(); closeModal(false); }
      else if (!info && e.key === 'Enter' && e.target.tagName !== 'BUTTON') { e.preventDefault(); closeModal(true); }
    };
    document.addEventListener('keydown', modalKeyHandler);
  });
}

function closeModal(result) {
  const backdrop = document.getElementById('modalBackdrop');
  if (backdrop) backdrop.style.display = 'none';
  // Reset confirm button visibility so subsequent confirm() calls show it again.
  const confirmBtn = document.getElementById('modalConfirmBtn');
  if (confirmBtn) confirmBtn.style.display = '';
  if (modalKeyHandler) {
    document.removeEventListener('keydown', modalKeyHandler);
    modalKeyHandler = null;
  }
  if (modalResolver) {
    const r = modalResolver;
    modalResolver = null;
    r(result);
  }
}

async function resetHistory() {
  const ok = await showConfirm({
    title: 'Reset history?',
    body: 'Streak, accuracy, category stats, and the session list will be cleared. Notes are kept.',
    confirmLabel: 'Reset',
    cancelLabel: 'Cancel',
    danger: true
  });
  if (!ok) return;
  state.streak = 0;
  state.totalAnswered = 0;
  state.totalCorrect = 0;
  state.categoryStats = {};
  state.sessions = [];
  state.historyResetAt = Date.now();
  saveState();
  updateStats();
  renderHistory();
}

function renderHistory() {
  const list = document.getElementById('historyList');
  const sessions = (state.sessions || []).filter(isValidSession);

  if (sessions.length === 0) {
    const resetAt = state.historyResetAt
      ? `<p class="history-empty-meta">Last reset: ${escapeHtml(fmtSessionDateTime(state.historyResetAt))}</p>`
      : '';
    const headline = state.historyResetAt
      ? 'No sessions since reset.'
      : 'Never taken a quiz yet.';
    list.innerHTML = `
      <p class="history-empty">${headline}</p>
      ${resetAt}
    `;
    return;
  }

  list.innerHTML = sessions.map((s, idx) => {
    const started = fmtSessionDateTime(s.started_at);
    const rawDur = s.ended_at ? fmtSessionDuration(s.ended_at - s.started_at) : '';
    const dur = s.abandoned
      ? `${rawDur} · abandoned`
      : (s.ended_at ? rawDur : 'in progress');
    const pct = s.total > 0 ? Math.round((s.correct / s.total) * 100) : 0;
    let summary;
    if (s.mode === 'custom') {
      const cats = s.config?.categories?.length || 0;
      const diffs = s.config?.difficulties?.length || 0;
      const catPart = cats === 0 ? 'all cats' : `${cats} cat${cats === 1 ? '' : 's'}`;
      const diffPart = diffs === 0 ? 'all diff' : `${diffs} diff`;
      summary = `Custom · ${catPart} · ${diffPart}`;
    } else {
      summary = 'Quick start';
    }

    const hist = Array.isArray(s.history) ? s.history : [];
    const inner = hist.length === 0
      ? '<p class="history-empty">No questions answered.</p>'
      : hist.map(h => {
          const sub = h.subcategory ? ` &middot; ${escapeHtml(h.subcategory)}` : '';
          return `
            <div class="history-item ${h.correct ? 'correct' : 'wrong'}">
              <div class="history-meta">
                <span>${escapeHtml(formatCategoryLabel(h.category))}${sub}</span>
                <span>${difficultyDots(h.difficulty)}</span>
              </div>
              <div class="history-question">${escapeHtml(h.question)}</div>
              <div class="history-result ${h.correct ? 'correct' : 'wrong'}">
                ${h.correct ? 'Correct' : 'Wrong (' + escapeHtml(h.selectedKey) + ' instead of ' + escapeHtml(h.answer) + ')'}
              </div>
            </div>
          `;
        }).join('');

    const canContinue = s.ended_at === null
      && !s.abandoned
      && Array.isArray(s.queue) && s.queue.length > 0
      && Array.isArray(s.history) && s.history.length < s.queue.length;
    const continueBtn = canContinue
      ? `<button class="session-continue" data-session-id="${escapeHtml(s.id)}" title="Resume this session">Continue</button>`
      : '';
    // Only sessions that actually ran (answered >=1 question) have a
    // useful recap to view. We also skip recap on a session that can
    // still be resumed, since Continue is the right action there.
    const canViewRecap = !canContinue && (s.total || 0) > 0;
    const recapBtn = canViewRecap
      ? `<button class="session-recap" data-session-id="${escapeHtml(s.id)}" title="View the final recap for this session">Recap</button>`
      : '';

    // Dropped <details>/<summary> for a plain div pair so we can animate
    // both expand AND collapse. <details> hides its non-summary children
    // outright when closed, which kills CSS transitions; this custom
    // toggle wraps the inner body in a grid container that animates
    // grid-template-rows 0fr <-> 1fr for a smooth open/close.
    const isOpen = idx === 0;
    return `
      <div class="session-block ${isOpen ? 'open' : ''}">
        <div class="session-summary" role="button" tabindex="0" aria-expanded="${isOpen}">
          <div class="session-summary-top">
            <span class="session-chevron" aria-hidden="true">&#9662;</span>
            <span class="session-time">${escapeHtml(started)}</span>
            <span class="session-score">${s.correct}/${s.total} &middot; ${pct}%</span>
            ${continueBtn}
            ${recapBtn}
            <button class="session-delete" data-session-id="${escapeHtml(s.id)}" title="Delete this session">&times;</button>
          </div>
          <div class="session-summary-sub">
            <span class="session-mode">${escapeHtml(summary)}</span>
            <span class="session-dur">${escapeHtml(dur)}</span>
          </div>
        </div>
        <div class="session-history-wrap">
          <div class="session-history">${inner}</div>
        </div>
      </div>
    `;
  }).join('');

  // Wire the custom toggle. Clicks on the inner buttons are excluded so
  // Continue / Recap / Delete don't accidentally fold the card open/closed.
  list.querySelectorAll('.session-block > .session-summary').forEach(summary => {
    const toggle = () => {
      const block = summary.parentElement;
      const nowOpen = !block.classList.contains('open');
      block.classList.toggle('open', nowOpen);
      summary.setAttribute('aria-expanded', String(nowOpen));
    };
    summary.addEventListener('click', (e) => {
      if (e.target.closest('.session-continue, .session-recap, .session-delete')) return;
      toggle();
    });
    summary.addEventListener('keydown', (e) => {
      if (e.key !== ' ' && e.key !== 'Enter') return;
      if (e.target.closest('.session-continue, .session-recap, .session-delete')) return;
      e.preventDefault();
      toggle();
    });
  });

  list.querySelectorAll('.session-delete').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      const id = btn.dataset.sessionId;
      state.sessions = (state.sessions || []).filter(s => s.id !== id);
      saveState();
      renderHistory();
    });
  });

  list.querySelectorAll('.session-continue').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      const id = btn.dataset.sessionId;
      const target = (state.sessions || []).find(s => s.id === id);
      if (!target) return;
      // Close the drawer first so the quiz is visible.
      if (document.getElementById('sidebar').classList.contains('open')) {
        toggleSidebar();
      }
      resumeSession(target);
    });
  });

  list.querySelectorAll('.session-recap').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      const id = btn.dataset.sessionId;
      const target = (state.sessions || []).find(s => s.id === id);
      if (!target) return;
      if (document.getElementById('sidebar').classList.contains('open')) {
        toggleSidebar();
      }
      renderRecap(target);
    });
  });
}

/* ---- Toolbox ---- */

function toggleToolbox() {
  if (state.toolbox?.open) closeToolbox();
  else openToolbox();
}

function openToolbox() {
  if (!state.toolbox) state.toolbox = { x: null, y: null, open: false, tab: 'calc' };
  state.toolbox.open = true;
  saveState();
  document.getElementById('toolbox').style.display = 'flex';
  const validTabs = ['calc', 'duration', 'scratch', 'cheat'];
  const tab = validTabs.includes(state.toolbox.tab) ? state.toolbox.tab : 'calc';
  setToolboxTab(tab);
}

function closeToolbox() {
  if (state.toolbox) state.toolbox.open = false;
  saveState();
  document.getElementById('toolbox').style.display = 'none';
}

function setToolboxTab(tab) {
  if (!state.toolbox) state.toolbox = { x: null, y: null, open: true, tab };
  state.toolbox.tab = tab;
  saveState();
  document.querySelectorAll('.toolbox-tab').forEach(b => {
    b.classList.toggle('active', b.dataset.tab === tab);
  });
  renderToolboxBody(tab);
}

function renderToolboxBody(tab) {
  const host = document.getElementById('toolboxBody');
  if (!host) return;
  host.innerHTML = '';
  if (tab === 'calc') renderCalc(host);
  else if (tab === 'duration') renderDuration(host);
  else if (tab === 'scratch') renderScratch(host);
  else if (tab === 'cheat') renderCheat(host);
}

function clampToolboxX(x) {
  const el = document.getElementById('toolbox');
  const w = el?.offsetWidth || 280;
  return Math.max(8, Math.min(x, window.innerWidth - w - 8));
}
function clampToolboxY(y) {
  const el = document.getElementById('toolbox');
  const h = el?.offsetHeight || 320;
  return Math.max(8, Math.min(y, window.innerHeight - h - 8));
}

function applyToolboxPosition() {
  const el = document.getElementById('toolbox');
  if (!el) return;
  // On narrow screens, let CSS place it (full-width bottom dock).
  if (window.innerWidth <= 480) {
    el.style.left = '';
    el.style.top = '';
    el.style.right = '';
    el.style.bottom = '';
    return;
  }
  const tb = state.toolbox || {};
  if (tb.x != null && tb.y != null) {
    el.style.left = clampToolboxX(tb.x) + 'px';
    el.style.top = clampToolboxY(tb.y) + 'px';
    el.style.right = 'auto';
    el.style.bottom = 'auto';
  }
}

function initToolboxDrag() {
  const box = document.getElementById('toolbox');
  const handle = document.getElementById('toolboxHeader');
  if (!box || !handle) return;
  let startX = 0, startY = 0, origX = 0, origY = 0, dragging = false;
  handle.addEventListener('mousedown', (e) => {
    if (window.innerWidth <= 480) return;
    if (e.target.closest('.toolbox-tab') || e.target.closest('.toolbox-close')) return;
    dragging = true;
    startX = e.clientX; startY = e.clientY;
    const rect = box.getBoundingClientRect();
    origX = rect.left; origY = rect.top;
    e.preventDefault();
  });
  window.addEventListener('mousemove', (e) => {
    if (!dragging) return;
    const nx = clampToolboxX(origX + (e.clientX - startX));
    const ny = clampToolboxY(origY + (e.clientY - startY));
    box.style.left = nx + 'px';
    box.style.top = ny + 'px';
    box.style.right = 'auto';
    box.style.bottom = 'auto';
  });
  window.addEventListener('mouseup', () => {
    if (!dragging) return;
    dragging = false;
    const rect = box.getBoundingClientRect();
    if (!state.toolbox) state.toolbox = {};
    state.toolbox.x = rect.left;
    state.toolbox.y = rect.top;
    saveState();
  });
}

/* ---- Duration math ---- */

function parseDuration(s) {
  if (!s || typeof s !== 'string') return null;
  // Replace `<n><suffix>` tokens with milliseconds in parens.
  const converted = s.replace(/(\d*\.?\d+)\s*(ms|s|m|h|d)\b/gi, (_, n, u) => {
    const mult = { ms: 1, s: 1000, m: 60000, h: 3600000, d: 86400000 }[u.toLowerCase()];
    return '(' + (Number(n) * mult) + ')';
  });
  const sanitized = converted.replace(/[^0-9+\-*/.() ]/g, '').trim();
  if (!sanitized) return null;
  try {
    const v = Function('"use strict"; return (' + sanitized + ')')();
    return typeof v === 'number' && Number.isFinite(v) ? v : null;
  } catch { return null; }
}

function formatDurationMs(ms) {
  if (ms == null || !Number.isFinite(ms)) return 'invalid';
  if (ms < 0) return '-' + formatDurationMs(-ms);
  let rem = Math.round(ms);
  const d = Math.floor(rem / 86400000); rem -= d * 86400000;
  const h = Math.floor(rem / 3600000);  rem -= h * 3600000;
  const m = Math.floor(rem / 60000);    rem -= m * 60000;
  const s = Math.floor(rem / 1000);     rem -= s * 1000;
  const parts = [];
  if (d) parts.push(d + 'd');
  if (h) parts.push(h + 'h');
  if (m) parts.push(m + 'm');
  if (s) parts.push(s + 's');
  if (rem) parts.push(rem + 'ms');
  return parts.join(' ') || '0s';
}

function renderDuration(host) {
  host.innerHTML = `
    <div class="tool-duration">
      <input type="text" class="tool-input" id="durInput" placeholder="e.g. 30s + 5m*3 + 1h">
      <div class="tool-output">
        <div class="tool-output-row"><span>ms</span><span id="durMs">&mdash;</span></div>
        <div class="tool-output-row"><span>human</span><span id="durHuman">&mdash;</span></div>
      </div>
      <p class="tool-hint">Suffixes: ms s m h d. Example: <code>2h + 30m*3</code>.</p>
    </div>
  `;
  const inp = document.getElementById('durInput');
  const msEl = document.getElementById('durMs');
  const humanEl = document.getElementById('durHuman');
  const paint = () => {
    const ms = parseDuration(inp.value);
    msEl.textContent = ms == null ? '\u2014' : String(Math.round(ms));
    humanEl.textContent = ms == null ? '\u2014' : formatDurationMs(ms);
  };
  inp.addEventListener('input', paint);
  inp.focus();
}

/* ---- Timer ---- */

let timerState = { qStart: null, sessionStart: null };
// One-shot flag so showQuestion() leaves the resumed lap offset intact on
// the first render after resumeSession(); it's cleared immediately after.
let resumingSession = false;

let mainTimerHandle = null;
// Tick-level persist throttle: we only write currentQElapsedMs to storage
// every ~2s to keep localStorage writes cheap while still saving enough
// lap detail that a crash or accidental tab-close barely loses anything.
let lastLapPersistAt = 0;

// Persist the active session's current-question elapsed time so it survives
// tab close / refresh / resume. Force=true ignores the throttle (used for
// pagehide / visibilitychange where we may not get another tick).
function persistCurrentLap(force = false) {
  if (!timerState.qStart) return;
  const activeSess = state.sessions && state.sessions[0];
  if (!activeSess || activeSess.ended_at !== null) return;
  const now = Date.now();
  if (!force && now - lastLapPersistAt < 2000) return;
  activeSess.currentQElapsedMs = now - timerState.qStart;
  lastLapPersistAt = now;
  saveState();
}

function resetCurrentLap() {
  lastLapPersistAt = 0;
  const activeSess = state.sessions && state.sessions[0];
  if (activeSess && activeSess.ended_at === null && activeSess.currentQElapsedMs) {
    activeSess.currentQElapsedMs = 0;
    saveState();
  }
}

function startMainTimer() {
  if (mainTimerHandle) return;
  const tick = () => {
    const qEl = document.getElementById('statTime');
    const sEl = document.getElementById('statSession');
    if (!qEl && !sEl) return;
    const qMs = timerState.qStart ? Date.now() - timerState.qStart : 0;
    const sMs = timerState.sessionStart ? Date.now() - timerState.sessionStart : 0;
    if (qEl) qEl.textContent = fmtSessionDuration(qMs);
    if (sEl) sEl.textContent = fmtSessionDuration(sMs);
    persistCurrentLap();
  };
  tick();
  mainTimerHandle = setInterval(tick, 500);
}

function stopMainTimer(resetSession = false) {
  if (mainTimerHandle) {
    clearInterval(mainTimerHandle);
    mainTimerHandle = null;
  }
  const qEl = document.getElementById('statTime');
  if (qEl) qEl.textContent = '0s';
  if (resetSession) {
    const sEl = document.getElementById('statSession');
    if (sEl) sEl.textContent = '0s';
  }
}

let scratchText = ''; // per-question scratch, resets in showQuestion

/* ---- Scratchpad ---- */

function renderScratch(host) {
  host.innerHTML = `
    <div class="tool-scratch">
      <textarea class="tool-textarea" id="scratchArea"
                placeholder="Scratch space. Clears each question."></textarea>
      <div class="tool-actions">
        <button class="btn btn-ghost" id="scratchSaveBtn" title="Save to question note">Save to note</button>
      </div>
    </div>
  `;
  const ta = document.getElementById('scratchArea');
  ta.value = scratchText;
  ta.addEventListener('input', () => { scratchText = ta.value; });
  document.getElementById('scratchSaveBtn').addEventListener('click', saveScratchToNote);
}

function saveScratchToNote() {
  if (!scratchText.trim()) return;
  const q = questions[currentIndex];
  if (!q) return;
  const key = hashQuestion(q.question);
  const prev = state.questionNotes[key];
  const now = Date.now();
  state.questionNotes[key] = {
    hash: key,
    note: prev ? (prev.note + '\n\n' + scratchText) : scratchText,
    question: q.question,
    source_doc: q.source_doc || '',
    category: q.category || '',
    created_at: prev?.created_at ?? now,
    updated_at: now
  };
  saveState();
  const btn = document.getElementById('scratchSaveBtn');
  if (btn) {
    const orig = btn.textContent;
    btn.textContent = 'Saved \u2713';
    setTimeout(() => { if (btn) btn.textContent = orig; }, 1200);
  }
}

/* ---- Cheat sheet ---- */

const CHEAT_SHEET = [
  { topic: 'Default Activity timeouts', body: 'ScheduleToCloseTimeout: none by default. You must set StartToCloseTimeout OR ScheduleToCloseTimeout.' },
  { topic: 'Default RetryPolicy', body: 'InitialInterval=1s, BackoffCoefficient=2.0, MaximumInterval=100*InitialInterval, MaximumAttempts=unlimited.' },
  { topic: 'Parent Close Policies', body: 'TERMINATE (default), ABANDON (orphan child), REQUEST_CANCEL (best-effort cancel).' },
  { topic: 'Signals vs Updates', body: 'Signals are async, no return value. Updates are sync, return a value, can validate input before accepting.' },
  { topic: 'Deterministic constraints', body: 'No time.Now(), no random, no network, no goroutines outside workflow.Go. Use workflow.Now, workflow.SideEffect.' },
  { topic: 'Workflow replay', body: 'Replay must match event history. Non-deterministic changes break replay. Use versioning (GetVersion) for migrations.' },
  { topic: 'Worker task queues', body: 'Workers poll a named task queue. Activities and workflows can be on different queues. Sticky queue for workflow tasks.' },
  { topic: 'Heartbeat timeout', body: 'For long activities, set HeartbeatTimeout and call RecordHeartbeat periodically. If missed, activity task fails.' },
  { topic: 'Search attributes', body: 'Typed key/value on a workflow. Indexable via ElasticSearch / SQL visibility. Mutable via UpsertSearchAttributes.' },
  { topic: 'Continue-As-New', body: 'Resets workflow history to keep it small. Same workflow ID, new run ID. Common for long-running cron-like workflows.' },
  { topic: 'Side effects', body: 'workflow.SideEffect captures a value once, replays deterministically. workflow.MutableSideEffect for values that can change.' },
  { topic: 'Nexus operations', body: 'Cross-namespace / cross-service calls over a typed contract. Sync or async. Use endpoints to target.' },
  { topic: 'Cron schedules', body: 'Run workflows on cron. Use ScheduleWorkflowOptions or the newer Schedules API. Schedules support pausing, backfill, jitter.' },
  { topic: 'Task timeouts', body: 'Workflow task timeout: time to complete a single task. Default 10s. Activity task timeouts are separate.' },
  { topic: 'Visibility queries', body: 'SQL-ish: SELECT * FROM workflows WHERE Status="Completed" ORDER BY StartTime. Use search attributes for custom fields.' }
];

function renderCheat(host) {
  host.innerHTML = `
    <div class="tool-cheat">
      <input type="text" class="tool-input" id="cheatQuery" placeholder="Search...">
      <div class="tool-cheat-list" id="cheatList"></div>
    </div>
  `;
  const list = document.getElementById('cheatList');
  const q = document.getElementById('cheatQuery');
  const paint = () => {
    const term = (q.value || '').toLowerCase().trim();
    const filtered = term
      ? CHEAT_SHEET.filter(e => e.topic.toLowerCase().includes(term) || e.body.toLowerCase().includes(term))
      : CHEAT_SHEET;
    list.innerHTML = filtered.length === 0
      ? '<p class="history-empty">No matches.</p>'
      : filtered.map(e => `
          <details class="cheat-item">
            <summary>${escapeHtml(e.topic)}</summary>
            <div class="cheat-body">${escapeHtml(e.body)}</div>
          </details>
        `).join('');
  };
  paint();
  q.addEventListener('input', paint);
  q.focus();
}

function renderCalc(host) {
  host.innerHTML = `
    <div class="tool-calc">
      <input type="text" class="calc-display" id="calcDisplay" readonly>
      <div class="calc-buttons" id="calcButtons">
        <button data-calc="7">7</button>
        <button data-calc="8">8</button>
        <button data-calc="9">9</button>
        <button data-calc="/">/</button>
        <button data-calc="4">4</button>
        <button data-calc="5">5</button>
        <button data-calc="6">6</button>
        <button data-calc="*">*</button>
        <button data-calc="1">1</button>
        <button data-calc="2">2</button>
        <button data-calc="3">3</button>
        <button data-calc="-">-</button>
        <button data-calc="0">0</button>
        <button data-calc=".">.</button>
        <button id="calcEvalBtn">=</button>
        <button data-calc="+">+</button>
        <button id="calcClearBtn" class="calc-clear">C</button>
      </div>
    </div>
  `;
  document.getElementById('calcButtons').addEventListener('click', (e) => {
    if (e.target.dataset.calc) {
      document.getElementById('calcDisplay').value += e.target.dataset.calc;
    }
  });
  document.getElementById('calcEvalBtn').addEventListener('click', () => {
    const d = document.getElementById('calcDisplay');
    try {
      const s = d.value.replace(/[^0-9+\-*/.() ]/g, '');
      d.value = Function('"use strict"; return (' + s + ')')();
    } catch { d.value = 'Error'; }
  });
  document.getElementById('calcClearBtn').addEventListener('click', () => {
    document.getElementById('calcDisplay').value = '';
  });
}

/* ---- Boot ---- */

// Force-persist the current question's lap time right before we lose control
// of the page. pagehide fires on close/navigation (more reliable than
// beforeunload on mobile); visibilitychange catches tab-backgrounding on
// iOS Safari where pagehide sometimes skips.
window.addEventListener('pagehide', () => persistCurrentLap(true));
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') persistCurrentLap(true);
});

/* ---- Pull-to-refresh (mobile) ----
   Lets the user pull down from the top of the page to reload. We replace
   the browser-native pull gesture (which varies across mobile browsers)
   with a consistent custom one that drives the #ptrIndicator. The pull is
   only active when the page is already scrolled to the top, the initial
   touch clearly moves downward, and we have touch support at all. */
(function initPullToRefresh() {
  if (!('ontouchstart' in window)) return;
  const THRESHOLD = 70; // px the user must pull past to trigger reload
  const MAX_PULL = 130; // clamp so the indicator never escapes the viewport
  const DRAG_RATIO = 0.5; // rubber-band: indicator moves at half finger speed

  let startY = 0;
  let pulling = false;
  let pulled = 0;
  let indicator = null;

  const getIndicator = () => indicator || (indicator = document.getElementById('ptrIndicator'));

  const show = (y) => {
    const el = getIndicator();
    if (!el) return;
    // y here is "pull distance"; nudge indicator down as we pull, capping at 20px
    // above the indicator's resting position so it feels anchored.
    const visible = Math.min(y - 40, 20);
    el.style.transform = `translate(-50%, ${visible}px)`;
    el.style.opacity = String(Math.min(1, y / THRESHOLD));
  };

  const hide = () => {
    const el = getIndicator();
    if (!el) return;
    el.style.transition = 'transform 0.2s ease, opacity 0.2s ease';
    el.style.transform = 'translate(-50%, -100%)';
    el.style.opacity = '0';
    setTimeout(() => { el.style.transition = ''; }, 220);
  };

  document.addEventListener('touchstart', (e) => {
    if (window.scrollY > 0) return;
    startY = e.touches[0].clientY;
    pulling = true;
    pulled = 0;
  }, { passive: true });

  document.addEventListener('touchmove', (e) => {
    if (!pulling) return;
    // If the page scrolled (e.g. user released near top then kept going),
    // bail and restore the indicator.
    if (window.scrollY > 0) { pulling = false; hide(); return; }
    const dy = e.touches[0].clientY - startY;
    if (dy <= 0) return; // finger moving up or sideways; let native scroll handle it
    // Take over the gesture so the browser's own pull-to-refresh doesn't fire.
    e.preventDefault();
    pulled = Math.min(MAX_PULL, dy * DRAG_RATIO);
    show(pulled);
  }, { passive: false });

  document.addEventListener('touchend', () => {
    if (!pulling) return;
    pulling = false;
    if (pulled >= THRESHOLD) {
      const el = getIndicator();
      // Flush any in-progress lap time so reload picks up where we left off.
      persistCurrentLap(true);
      if (el) el.classList.add('refreshing');
      // Tiny delay so the user sees the spinner lock in before the reload wipes it.
      setTimeout(() => window.location.reload(), 150);
      return;
    }
    hide();
  }, { passive: true });

  document.addEventListener('touchcancel', () => {
    if (!pulling) return;
    pulling = false;
    hide();
  }, { passive: true });
})();

init();
