package scraper

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func ScraperWorkflow(ctx workflow.Context, _ string) (int, error) {
	logger := workflow.GetLogger(ctx)

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
		InitialInterval: 2 * time.Second,
	}

	fetchOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	processOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	// Step 1: Download docs repo tarball from GitHub
	logger.Info("Fetching docs from GitHub")
	fetchCtx := workflow.WithActivityOptions(ctx, fetchOpts)
	var docsPath string
	if err := workflow.ExecuteActivity(fetchCtx, "FetchDocsRepo").Get(ctx, &docsPath); err != nil {
		return 0, err
	}
	logger.Info("Docs fetched", "path", docsPath)

	// Step 2: Read MDX files and write bucket text files
	processCtx := workflow.WithActivityOptions(ctx, processOpts)
	var processResult string
	if err := workflow.ExecuteActivity(processCtx, "ReadLocalDocs", docsPath).Get(ctx, &processResult); err != nil {
		return 0, err
	}

	logger.Info("Processing complete", "result", processResult)
	return 0, nil
}
