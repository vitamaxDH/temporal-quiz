# UI/UX Improvement Ideas: temporal.quiz

---

## 1. Question Transitions & Animations
**What:** Smooth card flip/slide when moving to next question. Right now the card just swaps instantly.
**How:** CSS `transform: translateX` slide-out on current card, slide-in on next. Or a subtle flip animation (like a flashcard).
**Why:** Makes the experience feel polished, not abrupt. One of the most impactful single changes.

---

## 2. Answer Feedback Micro-interactions
**What:** When you pick the correct answer, a brief green pulse/flash. Wrong answer: red shake.
**How:** CSS `@keyframes shake` on wrong, `@keyframes pulse` on correct answer element.
**Why:** Visceral feedback makes the result feel earned. Currently it just goes green/red statically.

---

## 3. Streak Celebration
**What:** When streak hits 5, 10, 25 etc. — a burst animation in the header. Could be confetti SVG particles or a glowing streak counter.
**How:** JS trigger + CSS animation injected into the DOM on milestone.
**Why:** Gamification hook. Makes people want to keep going.

---

## 4. Score / XP Summary Screen
**What:** After finishing a category (going through all questions), show a summary card instead of wrapping silently.
**Layout:**
```
┌─────────────────────────────┐
│  Features / Workflows        │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━  │
│   8 / 13  correct   62%      │
│   Best difficulty: hard      │
│   Accuracy trend: ↑          │
│  [Retry]   [Next Category]   │
└─────────────────────────────┘
```
**Why:** Closure. Right now it just reshuffles and starts over silently.

---

## 5. Per-difficulty Progress Bars
**What:** In the stats area or somewhere visible, show your accuracy broken down by difficulty (easy/med/hard/nightmare) as small bars or donut segments.
**Why:** Shows you exactly where your weak spots are, not just overall accuracy.

---

## 6. Keyboard Shortcut Cheat Sheet
**What:** A small `?` button (or press `?`) that shows a tooltip/overlay:
```
A B C D  — select answer
Enter    — submit / next
→        — skip
H        — toggle history
Calc     — toggle calculator
```
**Why:** Power users (devs) will love it. Most won't know keyboard shortcuts exist.

---

## 7. Dark/Light Mode Toggle
**What:** A sun/moon toggle in the header. Light mode uses off-white background with darker text.
**Why:** Some people genuinely prefer light mode. Easy win for accessibility.

---

## 8. Question Bookmark / Save for Later
**What:** A bookmark icon on each question card. Saved questions accessible from the sidebar alongside history.
**How:** Store `bookmarks: [questionId, ...]` in localStorage. Sidebar gets a "Bookmarks" tab alongside "History".
**Why:** When you encounter a hard question mid-session, you want to come back to it.

---

## 9. Category Progress Indicators on Pills
**What:** Small accuracy badge on each category pill showing your % correct.
```
[ Features / Workflows  82% ]  ← green tint
[ Features / Nexus      34% ]  ← red tint
```
**How:** `state.categoryStats` already has this data. Render it inline on pill.
**Why:** Instantly shows which categories need work without opening any sidebar.

---

## 10. Onboarding Empty State
**What:** First-time visitors see "Select a category to begin" which is fine, but could be better. A subtle guided arrow pointing at the category pills, or a first-run tooltip sequence.
**Why:** New users might not know to click a pill to start.

---

## 11. Mobile Swipe Gestures
**What:** Swipe left to skip, swipe right to go back, swipe up to reveal explanation.
**How:** Touch event listeners on `.question-card`.
**Why:** The UI is responsive but feels desktop-first. Swipe makes it native-feeling on mobile.

---

## 12. Accuracy Sparkline in History Sidebar
**What:** A tiny sparkline chart at the top of the History sidebar showing accuracy trend over last 20 answers (up/down based on correct/wrong).
**How:** SVG path generated from `state.history` array, inline in the sidebar header.
**Why:** At a glance you see if you're improving or declining.

---

## Priority ranking

| # | Feature | Effort | Impact |
|---|---------|--------|--------|
| 2 | Answer feedback micro-interactions | Low | High |
| 1 | Question transitions | Low | High |
| 9 | Category pill accuracy badges | Low | High |
| 4 | End-of-category summary | Medium | High |
| 6 | Keyboard shortcut overlay | Low | Medium |
| 3 | Streak celebration | Low | Medium |
| 8 | Bookmark questions | Medium | Medium |
| 5 | Per-difficulty progress bars | Medium | Medium |
| 12 | Accuracy sparkline | Medium | Medium |
| 11 | Mobile swipe gestures | Medium | Medium |
| 7 | Dark/light mode | High | Low-Medium |
| 10 | Onboarding tooltips | Medium | Low |
