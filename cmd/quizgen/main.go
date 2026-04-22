package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"temporal-quiz/config"
	"temporal-quiz/quiz"

	"go.temporal.io/sdk/client"
)

func main() {
	easy := flag.Int("easy", 3, "number of easy questions per category (max 30)")
	med := flag.Int("med", 4, "number of med questions per category (max 30)")
	hard := flag.Int("hard", 4, "number of hard questions per category (max 30)")
	nightmare := flag.Int("nightmare", 2, "number of nightmare questions per category (max 30)")
	skipEval := flag.Bool("skip-eval", false, "skip AI quality evaluation")
	flag.Parse()

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

	var result int
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nQuiz generation complete! %d questions generated.\n", result)
}
