package main

import (
	"context"
	"fmt"
	"log"

	"temporal-quiz/config"
	"temporal-quiz/scraper"

	"go.temporal.io/sdk/client"
)

func main() {
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	fmt.Println("Triggering ScraperWorkflow (fetching docs from GitHub)...")

	we, err := c.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        "temporal-docs-scraper-workflow",
			TaskQueue: scraper.TaskQueue,
		},
		scraper.ScraperWorkflow,
		"",
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Workflow started: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result int
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nWorkflow completed! Docs fetched and bucketed.\n")
}
