# Quiz Evaluation: AI Quality Check After Generation

**Goal:** Add a post-generation step that uses Claude to evaluate quiz quality, flag bad questions, and optionally filter them out before they reach the web UI.

---

## Why

Some generated questions are low quality:
- Ambiguous answers where multiple choices could be correct
- Questions that test memorization instead of understanding
- Explanations that don't actually explain the right answer
- Questions generated from thin/index content (mostly fixed, but edge cases remain)
- Difficulty mismatch (an "easy" question that's actually hard, or vice versa)

## Approach: EvaluateQuiz Activity

A new activity that takes generated questions and asks Claude to score them. Runs after `GenerateQuiz` but before `WriteQuizFiles`.

### Evaluation Prompt

```
You are a quiz quality evaluator. Score each question on these criteria (1-5):

1. CLARITY: Is the question unambiguous? Is there exactly one correct answer?
2. ACCURACY: Is the stated answer actually correct based on the source material?
3. DIFFICULTY_FIT: Does the difficulty label match the actual difficulty?
4. EXPLANATION: Does the explanation teach something useful?
5. RELEVANCE: Is this a question worth asking? Does it test real understanding?

For each question, return:
- scores (object with the 5 criteria)
- pass (boolean: true if all scores >= 3)
- feedback (string: one sentence on why it failed, or empty if passed)
```

### Data Flow

```
Current:  GenerateQuiz -> WriteQuizFiles
Proposed: GenerateQuiz -> EvaluateQuiz -> WriteQuizFiles
```

Only questions that pass evaluation get written. Failed questions are logged for review.

### New Types

```go
type EvalResult struct {
    QuestionID string         `json:"question_id"`
    Scores     map[string]int `json:"scores"`      // clarity, accuracy, difficulty_fit, explanation, relevance
    Pass       bool           `json:"pass"`
    Feedback   string         `json:"feedback"`
}

type EvalOutput struct {
    Passed []QuizQuestion
    Failed []QuizQuestion
    Results []EvalResult
}
```

### New Activity: `EvaluateQuiz`

```go
func (a *QuizActivities) EvaluateQuiz(ctx context.Context, questions []QuizQuestion) (EvalOutput, error) {
    // Batch questions (10 at a time to fit in context)
    // Send each batch to Claude with the evaluation prompt
    // Parse scores, filter pass/fail
    // Return both lists + detailed results
}
```

### Workflow Change

In `QuizGeneratorWorkflow`, add eval step after fan-in:

```go
// Step 3: Fan-in results
var allQuestions []QuizQuestion
// ... existing fan-in code ...

// Step 3.5: Evaluate quality
evalCtx := workflow.WithActivityOptions(ctx, evalOpts)
var evalOutput EvalOutput
if err := workflow.ExecuteActivity(evalCtx, "EvaluateQuiz", allQuestions).Get(ctx, &evalOutput); err != nil {
    // Eval failure is non-fatal, use all questions
    logger.Warn("Quiz evaluation failed, using unfiltered questions", "error", err)
} else {
    allQuestions = evalOutput.Passed
    logger.Info("Quiz evaluation complete", "passed", len(evalOutput.Passed), "failed", len(evalOutput.Failed))
}

// Step 4: Write quiz files (only passed questions)
```

### CLI Flag

Add `-skip-eval` flag to skip evaluation (useful for faster iteration during dev).

### Cost Estimate

- ~15 categories x 13 questions = ~195 questions
- Batched 10 at a time = ~20 Claude API calls for evaluation
- Each eval call is small (just scores, not generating new content)
- Roughly 10-20% of the cost of the generation step itself

---

## Files Changed

| File | Changes |
|------|---------|
| `quiz/types.go` | Add `EvalResult`, `EvalOutput` |
| `quiz/prompt.go` | Add `EvalPrompt()` |
| `quiz/activities.go` | Add `EvaluateQuiz` activity |
| `quiz/workflow.go` | Add eval step between fan-in and write |
| `cmd/quizgen/main.go` | Add `-skip-eval` flag |
| `cmd/localgen/main.go` | Add `-skip-eval` flag |
| `quiz/prompt_test.go` | Test for EvalPrompt |
| `quiz/activities_test.go` | Test for EvaluateQuiz |

---

## Open Questions

1. **Threshold:** Should all 5 criteria need >= 3, or use an average score?
2. **Regeneration:** When questions fail, should we try to regenerate replacements to maintain target count per category? (Adds complexity and cost)
3. **Report:** Should we write a `eval_report.json` alongside the quizzes so you can review what got filtered?
