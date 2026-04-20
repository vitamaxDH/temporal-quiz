package quiz

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// QuizGeneratorWorkflow generates quiz questions for all doc categories.
// params controls difficulty mix, question counts, and category filters.
func QuizGeneratorWorkflow(ctx workflow.Context, params QuizGenParams) (int, error) {
	logger := workflow.GetLogger(ctx)

	// Apply defaults and caps
	if params.EasyCount <= 0 {
		params.EasyCount = 3
	}
	if params.MedCount <= 0 {
		params.MedCount = 4
	}
	if params.HardCount <= 0 {
		params.HardCount = 4
	}
	if params.NightmareCount <= 0 {
		params.NightmareCount = 2
	}
	if params.EasyCount > 30 {
		params.EasyCount = 30
	}
	if params.MedCount > 30 {
		params.MedCount = 30
	}
	if params.HardCount > 30 {
		params.HardCount = 30
	}
	if params.NightmareCount > 30 {
		params.NightmareCount = 30
	}

	listOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	}

	genOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
			InitialInterval: 5 * time.Second,
		},
	}

	writeOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	}

	// Step 1: List all bucket files
	listCtx := workflow.WithActivityOptions(ctx, listOpts)
	var buckets []GenerateQuizInput
	if err := workflow.ExecuteActivity(listCtx, "ListBuckets").Get(ctx, &buckets); err != nil {
		return 0, err
	}

	// Filter categories if specified
	if len(params.Categories) > 0 {
		catSet := make(map[string]bool)
		for _, c := range params.Categories {
			catSet[c] = true
		}
		var filtered []GenerateQuizInput
		for _, b := range buckets {
			if catSet[b.Category] {
				filtered = append(filtered, b)
			}
		}
		buckets = filtered
	}

	logger.Info("Generating quizzes", "categories", len(buckets), "easy", params.EasyCount, "med", params.MedCount, "hard", params.HardCount, "nightmare", params.NightmareCount)

	// Step 2: Fan-out quiz generation per category
	// Priority categories get a larger pool so more questions survive eval.
	genCtx := workflow.WithActivityOptions(ctx, genOpts)
	futures := make([]workflow.Future, len(buckets))
	for i, bucket := range buckets {
		mult := 1
		if IsPriorityCategory(bucket.Category) {
			mult = PriorityCountMultiplier
		}
		bucket.EasyCount = params.EasyCount * mult
		bucket.MedCount = params.MedCount * mult
		bucket.HardCount = params.HardCount * mult
		bucket.NightmareCount = params.NightmareCount * mult
		buckets[i] = bucket
		futures[i] = workflow.ExecuteActivity(genCtx, "GenerateQuiz", bucket)
	}

	// Step 3: Fan-in results, keeping per-category bucketing for later recovery.
	var allQuestions []QuizQuestion
	preEvalByCategory := make(map[string][]QuizQuestion)
	for i, f := range futures {
		var questions []QuizQuestion
		if err := f.Get(ctx, &questions); err != nil {
			logger.Warn("Failed to generate quiz", "category", buckets[i].Category, "error", err)
			continue
		}
		preEvalByCategory[buckets[i].Category] = questions
		allQuestions = append(allQuestions, questions...)
	}

	if len(allQuestions) == 0 {
		return 0, nil
	}

	// Step 3.5: Evaluate quality (unless skipped)
	if !params.SkipEval {
		evalOpts := workflow.ActivityOptions{
			StartToCloseTimeout: 10 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 2,
				InitialInterval: 5 * time.Second,
			},
		}
		evalCtx := workflow.WithActivityOptions(ctx, evalOpts)
		var evalOutput EvalOutput
		if err := workflow.ExecuteActivity(evalCtx, "EvaluateQuiz", allQuestions).Get(ctx, &evalOutput); err != nil {
			logger.Warn("Quiz evaluation failed, using unfiltered questions", "error", err)
		} else {
			logger.Info("Quiz evaluation complete", "passed", len(evalOutput.Passed), "failed", len(evalOutput.Failed))
			allQuestions = evalOutput.Passed

			// Recovery: if a priority category lost ALL its questions to the
			// eval filter, restore its pre-eval set so daily runs always show
			// the core topics even when eval is strict.
			passedCats := make(map[string]bool)
			for _, q := range allQuestions {
				passedCats[q.Category] = true
			}
			for _, pri := range PriorityCategories {
				if passedCats[pri] {
					continue
				}
				recovered := preEvalByCategory[pri]
				if len(recovered) == 0 {
					continue
				}
				logger.Warn("Priority category lost all questions to eval; restoring pre-eval set",
					"category", pri, "restored", len(recovered))
				allQuestions = append(allQuestions, recovered...)
			}
		}
	}

	// Step 4: Write quiz files
	writeCtx := workflow.WithActivityOptions(ctx, writeOpts)
	if err := workflow.ExecuteActivity(writeCtx, "WriteQuizFiles", allQuestions).Get(ctx, nil); err != nil {
		return 0, err
	}

	logger.Info("Quiz generation complete", "totalQuestions", len(allQuestions))
	return len(allQuestions), nil
}

// DailyPipelineWorkflow orchestrates docs fetching then quiz generation.
func DailyPipelineWorkflow(ctx workflow.Context, params QuizGenParams) (string, error) {
	logger := workflow.GetLogger(ctx)

	// Run scraper as child workflow (fetches docs from GitHub)
	scraperOpts := workflow.ChildWorkflowOptions{
		WorkflowID: "daily-scraper",
	}
	scraperCtx := workflow.WithChildOptions(ctx, scraperOpts)

	logger.Info("Starting docs fetch")
	if err := workflow.ExecuteChildWorkflow(scraperCtx, "ScraperWorkflow", "").Get(ctx, nil); err != nil {
		return "", err
	}
	logger.Info("Docs fetch complete")

	// Run quiz generator as child workflow
	quizOpts := workflow.ChildWorkflowOptions{
		WorkflowID: "daily-quiz-generator",
	}
	quizCtx := workflow.WithChildOptions(ctx, quizOpts)

	var quizResult int
	if err := workflow.ExecuteChildWorkflow(quizCtx, "QuizGeneratorWorkflow", params).Get(ctx, &quizResult); err != nil {
		return "", err
	}
	logger.Info("Quiz generation complete", "questionsGenerated", quizResult)

	return fmt.Sprintf("Generated %d questions", quizResult), nil
}
