package main

import (
	"context"
	"fmt"
	"log"

	"temporal-quiz/config"
	"temporal-quiz/quiz"

	"go.temporal.io/sdk/client"
)

func main() {
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	params := quiz.QuizGenParams{
		EasyCount:      3,
		MedCount:       4,
		HardCount:      4,
		NightmareCount: 2,
	}

	fmt.Println("Starting DailyPipelineWorkflow...")

	we, err := c.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        "daily-pipeline",
			TaskQueue: quiz.TaskQueue,
		},
		quiz.DailyPipelineWorkflow,
		params,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Workflow started: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result string
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nPipeline complete! %s\n", result)
}
