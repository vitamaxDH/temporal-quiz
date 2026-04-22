package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"temporal-quiz/config"
	"temporal-quiz/quiz"
	"temporal-quiz/scraper"

	"go.temporal.io/sdk/client"
)

func main() {
	docsDir := flag.String("docs", "", "local docs directory path (skip GitHub fetch)")
	easy := flag.Int("easy", 3, "number of easy questions per category (max 30)")
	med := flag.Int("med", 4, "number of med questions per category (max 30)")
	hard := flag.Int("hard", 4, "number of hard questions per category (max 30)")
	nightmare := flag.Int("nightmare", 2, "number of nightmare questions per category (max 30)")
	skipEval := flag.Bool("skip-eval", false, "skip AI quality evaluation")
	flag.Parse()

	a := &scraper.Activities{
		Client: &http.Client{},
	}

	var docsPath string
	if *docsDir != "" {
		// Local mode: use provided directory
		fmt.Printf("Reading local docs from %s...\n", *docsDir)
		if _, err := os.Stat(*docsDir); os.IsNotExist(err) {
			log.Fatalf("Docs directory does not exist: %s", *docsDir)
		}
		docsPath = *docsDir
	} else {
		// GitHub mode: download public repo tarball
		fmt.Println("Fetching docs from GitHub...")
		var err error
		docsPath, err = a.FetchDocsRepo(context.Background())
		if err != nil {
			log.Fatalf("Failed to fetch docs repo: %v", err)
		}
		defer os.RemoveAll(docsPath) // clean up temp dir
		fmt.Printf("Docs extracted to %s\n", docsPath)
	}

	// Read docs and write bucket files
	result, err := a.ReadLocalDocs(context.Background(), docsPath)
	if err != nil {
		log.Fatalf("Failed to read docs: %v", err)
	}
	fmt.Println(result)

	// Start quiz generation workflow
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	params := quiz.QuizGenParams{
		EasyCount:      *easy,
		MedCount:       *med,
		HardCount:      *hard,
		NightmareCount: *nightmare,
		SkipEval:       *skipEval,
	}

	fmt.Printf("Starting QuizGeneratorWorkflow (easy=%d, med=%d, hard=%d, nightmare=%d)...\n",
		params.EasyCount, params.MedCount, params.HardCount, params.NightmareCount)

	we, err := c.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        "quiz-generator",
			TaskQueue: quiz.TaskQueue,
		},
		quiz.QuizGeneratorWorkflow,
		params,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Workflow started: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var quizResult int
	if err := we.Get(context.Background(), &quizResult); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nQuiz generation complete! %d questions generated.\n", quizResult)
}
