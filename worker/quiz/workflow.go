package quiz

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CategoryPipelineWorkflow runs GenerateQuiz for a single category and, as
// soon as those questions land, fans out EvaluateQuiz batches for just that
// category. Running this per-category means evaluation for finished
// categories overlaps with still-running generation elsewhere, shaving the
// total wall-clock down to max(gen_i + eval_i) instead of
// max(gen) + max(eval_batches).
func CategoryPipelineWorkflow(ctx workflow.Context, input CategoryPipelineInput) (CategoryPipelineResult, error) {
	logger := workflow.GetLogger(ctx)

	genOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
			InitialInterval: 5 * time.Second,
		},
	}
	evalOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
			InitialInterval: 5 * time.Second,
		},
	}

	result := CategoryPipelineResult{Category: input.Bucket.Category}

	genCtx := workflow.WithActivityOptions(ctx, genOpts)
	var questions []QuizQuestion
	if err := workflow.ExecuteActivity(genCtx, "GenerateQuiz", input.Bucket).Get(ctx, &questions); err != nil {
		return result, err
	}
	result.PreEval = questions

	if input.SkipEval || len(questions) == 0 {
		result.Passed = questions
		result.PassedCount = len(questions)
		return result, nil
	}

	// Fan out eval batches for this category only. Each batch is its own
	// activity so Claude calls run in parallel.
	evalCtx := workflow.WithActivityOptions(ctx, evalOpts)
	var evalFutures []workflow.Future
	for i := 0; i < len(questions); i += EvalBatchSize {
		end := i + EvalBatchSize
		if end > len(questions) {
			end = len(questions)
		}
		evalFutures = append(evalFutures, workflow.ExecuteActivity(evalCtx, "EvaluateQuiz", questions[i:end]))
	}

	var passed []QuizQuestion
	passedCount, failedCount := 0, 0
	evalHadFailure := false
	for _, f := range evalFutures {
		var out EvalOutput
		if err := f.Get(ctx, &out); err != nil {
			logger.Warn("Eval batch failed, keeping questions unfiltered for this batch",
				"category", input.Bucket.Category, "error", err)
			evalHadFailure = true
			continue
		}
		passed = append(passed, out.Passed...)
		passedCount += len(out.Passed)
		failedCount += len(out.Failed)
	}

	if evalHadFailure && passedCount == 0 {
		// Eval fully failed for this category — fall back to the pre-eval set
		// so we don't lose the whole category on a transient Claude hiccup.
		logger.Warn("All eval batches failed for category; using unfiltered set",
			"category", input.Bucket.Category)
		result.Passed = questions
		result.PassedCount = len(questions)
		return result, nil
	}

	result.Passed = passed
	result.PassedCount = passedCount
	result.FailedCount = failedCount
	return result, nil
}

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

	// Step 2: Fan out one child workflow per category. Each child owns its
	// own gen → eval pipeline, so category A's eval can start the instant
	// category A's gen finishes without waiting for the rest.
	childFutures := make([]workflow.ChildWorkflowFuture, len(buckets))
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

		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("category-pipeline-%s-%s", bucket.Category, workflow.GetInfo(ctx).WorkflowExecution.RunID),
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 2,
			},
		}
		childCtx := workflow.WithChildOptions(ctx, childOpts)
		childFutures[i] = workflow.ExecuteChildWorkflow(childCtx, "CategoryPipelineWorkflow", CategoryPipelineInput{
			Bucket:   bucket,
			SkipEval: params.SkipEval,
		})
	}

	// Step 3: Fan in child workflow results.
	var allQuestions []QuizQuestion
	preEvalByCategory := make(map[string][]QuizQuestion)
	totalPassed, totalFailed := 0, 0
	for i, f := range childFutures {
		var res CategoryPipelineResult
		if err := f.Get(ctx, &res); err != nil {
			logger.Warn("Category pipeline failed", "category", buckets[i].Category, "error", err)
			continue
		}
		preEvalByCategory[res.Category] = res.PreEval
		allQuestions = append(allQuestions, res.Passed...)
		totalPassed += res.PassedCount
		totalFailed += res.FailedCount
	}

	if len(allQuestions) == 0 {
		return 0, nil
	}

	if !params.SkipEval {
		logger.Info("Quiz evaluation complete", "passed", totalPassed, "failed", totalFailed)

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
