package quiz

import "fmt"

func EasyPrompt(n int, bucketText string) string {
	return fmt.Sprintf(`You are a Temporal platform expert creating beginner-friendly quiz questions. Your goal is to help newcomers build foundational understanding of Temporal concepts.

RULES:
- Questions should test UNDERSTANDING of core concepts, not memorization
- Use simple, clear language with minimal jargon
- Wrong answers should be plausible but clearly distinguishable with basic knowledge
- The explanation should teach the concept from scratch, assuming the reader is new to Temporal
- Focus on "what" and "why" rather than edge cases or production gotchas
- Do NOT include code snippets or require SDK-specific knowledge

CHOICE QUALITY (critical — avoid these MCQ tells):
- All four choices (A, B, C, D) must be comparable in length and specificity. Aim for within ~20%% word count of each other.
- The correct answer must NOT be the longest, most hedged, most qualified, or most "textbook-sounding" option. Wrong answers should not be noticeably terser.
- Rotate which letter is correct across the question set so A/B/C/D are roughly evenly distributed. Do NOT default to a single letter.

TOPICAL SPREAD:
- If the documentation covers a core Temporal primitive (Workflows, Activities, Workers, Task Queues, Signals, Queries, Updates, Nexus, Data Converter, Retry Policies, Schedules, Child Workflows, Continue-As-New, Versioning, etc.), spread the questions across a diverse set of sub-topics — definition, lifecycle, common APIs, typical usage, and common pitfalls — rather than clustering on one narrow aspect.
- Operator-facing concepts matter too. When the docs touch on Namespaces, connectivity and TLS, mTLS client certificates, Temporal Cloud networking, private link / IP allowlists, cluster topology, task queue routing, visibility store, cross-namespace Nexus endpoints, dual-visibility migration, API keys, RBAC roles, or any other concern someone running Temporal in production would need to reason about, include at least one question on the operator angle.

Generate %d easy multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"A","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}

func MedPrompt(n int, bucketText string) string {
	return fmt.Sprintf(`You are a Temporal platform expert creating intermediate quiz questions. Your goal is to help engineers solidify their practical understanding of Temporal patterns and APIs.

RULES:
- Questions test PRACTICAL KNOWLEDGE of how Temporal features work
- May include simple code snippets (Go or Python) showing common patterns
- Wrong answers should represent reasonable misunderstandings, not absurd options
- The explanation should clarify the concept and mention when you would use this pattern in practice
- Focus on "how it works" and correct usage, not extreme edge cases
- Questions should be answerable by someone who has built a few Temporal workflows

CHOICE QUALITY (critical — avoid these MCQ tells):
- All four choices (A, B, C, D) must be comparable in length and specificity. Aim for within ~20%% word count of each other.
- The correct answer must NOT be the longest, most hedged, most qualified, or most "textbook-sounding" option. Wrong answers should not be noticeably terser.
- Rotate which letter is correct across the question set so A/B/C/D are roughly evenly distributed. Do NOT default to a single letter.

TOPICAL SPREAD:
- If the documentation covers a core Temporal primitive (Workflows, Activities, Workers, Task Queues, Signals, Queries, Updates, Nexus, Data Converter, Retry Policies, Schedules, Child Workflows, Continue-As-New, Versioning, etc.), spread the questions across a diverse set of sub-topics — definition, lifecycle, common APIs, typical usage, and common pitfalls — rather than clustering on one narrow aspect.
- Operator-facing concepts matter too. When the docs touch on Namespaces, connectivity and TLS, mTLS client certificates, Temporal Cloud networking, private link / IP allowlists, cluster topology, task queue routing, visibility store, cross-namespace Nexus endpoints, dual-visibility migration, API keys, RBAC roles, or any other concern someone running Temporal in production would need to reason about, include at least one question on the operator angle.

Generate %d medium-difficulty multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"C","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}

func HardPrompt(n int, bucketText string) string {
	return fmt.Sprintf(`You are a Temporal platform expert creating educational quiz questions that help engineers deeply understand Temporal. Your goal is to help people LEARN, not to trick them. Every question should teach something valuable about how Temporal works in production.

RULES:
- Every question describes a REAL-WORLD PRODUCTION SCENARIO
- Wrong answers represent COMMON MISCONCEPTIONS that engineers actually make
- The explanation is the most important part: explain WHY the correct answer is right, what the underlying Temporal design principle is, and what would go wrong if you assumed otherwise
- After answering, the reader should understand a concept they can apply to their own Temporal code
- Do NOT write definition/recall questions like "What is X?"
- Do NOT make questions tricky for the sake of being tricky

CHOICE QUALITY (critical — avoid these MCQ tells):
- All four choices (A, B, C, D) must be comparable in length and specificity. Aim for within ~20%% word count of each other.
- The correct answer must NOT be the longest, most hedged, most qualified, or most "textbook-sounding" option. Wrong answers should not be noticeably terser.
- Rotate which letter is correct across the question set so A/B/C/D are roughly evenly distributed. Do NOT default to a single letter.

TOPICAL SPREAD:
- If the documentation covers a core Temporal primitive (Workflows, Activities, Workers, Task Queues, Signals, Queries, Updates, Nexus, Data Converter, Retry Policies, Schedules, Child Workflows, Continue-As-New, Versioning, etc.), spread the questions across a diverse set of sub-topics — definition, lifecycle, common APIs, typical usage, and common pitfalls — rather than clustering on one narrow aspect.
- Operator-facing concepts matter too. When the docs touch on Namespaces, connectivity and TLS, mTLS client certificates, Temporal Cloud networking, private link / IP allowlists, cluster topology, task queue routing, visibility store, cross-namespace Nexus endpoints, dual-visibility migration, API keys, RBAC roles, or any other concern someone running Temporal in production would need to reason about, include at least one question on the operator angle.

Generate %d hard multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"D","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}

func NightmarePrompt(n int, bucketText string) string {
	return fmt.Sprintf(`You are a Temporal platform expert creating advanced educational quiz questions for experienced engineers. These questions teach how multiple Temporal features interact in complex production scenarios. The goal is growth, not gotchas.

RULES:
- Questions describe COMPLEX PRODUCTION SCENARIOS where 2-3 Temporal features interact (e.g., child workflows + signals + timeouts, versioning + continue-as-new)
- May include Go or Python code snippets showing real workflow/activity patterns
- Wrong answers represent things an engineer might reasonably believe before understanding the deeper behavior
- The explanation should go deep: explain the underlying design principle, connect it to broader Temporal architecture, and give the reader an insight they can apply beyond this specific question
- After reading the explanation, the engineer should think "I'm glad I learned that before hitting it in production"

CHOICE QUALITY (critical — avoid these MCQ tells):
- All four choices (A, B, C, D) must be comparable in length and specificity. Aim for within ~20%% word count of each other.
- The correct answer must NOT be the longest, most hedged, most qualified, or most "textbook-sounding" option. Wrong answers should not be noticeably terser.
- Rotate which letter is correct across the question set so A/B/C/D are roughly evenly distributed. Do NOT default to a single letter.

TOPICAL SPREAD:
- If the documentation covers a core Temporal primitive (Workflows, Activities, Workers, Task Queues, Signals, Queries, Updates, Nexus, Data Converter, Retry Policies, Schedules, Child Workflows, Continue-As-New, Versioning, etc.), spread the questions across a diverse set of sub-topics — definition, lifecycle, common APIs, typical usage, and common pitfalls — rather than clustering on one narrow aspect.
- Operator-facing concepts matter too. When the docs touch on Namespaces, connectivity and TLS, mTLS client certificates, Temporal Cloud networking, private link / IP allowlists, cluster topology, task queue routing, visibility store, cross-namespace Nexus endpoints, dual-visibility migration, API keys, RBAC roles, or any other concern someone running Temporal in production would need to reason about, include at least one question on the operator angle.

Generate %d nightmare-difficulty multiple-choice questions from this documentation:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question":"...","choices":[{"key":"A","text":"..."},{"key":"B","text":"..."},{"key":"C","text":"..."},{"key":"D","text":"..."}],"answer":"B","explanation":"...","source_doc":"filename.html"}]`, n, bucketText)
}

func EvalPrompt(questionsJSON string) string {
	return fmt.Sprintf(`You are a quiz quality evaluator for Temporal platform educational content. Evaluate each question on these criteria (score 1-5):

1. CLARITY: Is the question unambiguous? Is there exactly one clearly correct answer? Are the four choices comparable in length and specificity (no choice should be noticeably longer, more hedged, or more qualified than the others)?
2. ACCURACY: Is the stated correct answer actually correct?
3. DIFFICULTY_FIT: Does the difficulty label (easy/med/hard/nightmare) match the actual difficulty?
4. EXPLANATION: Does the explanation teach something useful and correctly explain why the answer is right?
5. RELEVANCE: Is this a question worth asking? Does it test real understanding rather than trivia?

A question PASSES if ALL scores are >= 3. Otherwise it FAILS.

Evaluate these questions:
%s

Return ONLY a JSON array matching this exact schema (no markdown fences, no extra text):
[{"question_id":"...","scores":{"clarity":4,"accuracy":5,"difficulty_fit":3,"explanation":4,"relevance":5},"pass":true,"feedback":""}]

For failed questions, include a brief feedback explaining why.`, questionsJSON)
}
