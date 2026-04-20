package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"temporal-quiz/config"
	"temporal-quiz/quiz"
	"temporal-quiz/scraper"

	"go.temporal.io/sdk/worker"
)

func main() {
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()
	fmt.Println("Connected to Temporal!")

	activities := &scraper.Activities{
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	w := worker.New(c, scraper.TaskQueue, worker.Options{})
	w.RegisterWorkflow(scraper.ScraperWorkflow)
	w.RegisterActivity(activities)

	outputDir := os.Getenv("TEMPORAL_QUIZ_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = quiz.DefaultOutputDir
	}
	quizActivities := &quiz.QuizActivities{
		HTTPClient: &http.Client{},
		OutputDir:  outputDir,
		BucketDir:  quiz.BucketDir,
		APIKey:     config.GetAnthropicKey(),
		Model:      config.GetLLMModel(),
	}

	w.RegisterWorkflow(quiz.QuizGeneratorWorkflow)
	w.RegisterWorkflow(quiz.DailyPipelineWorkflow)
	w.RegisterActivity(quizActivities)

	fmt.Printf("Starting worker on task queue '%s'...\n", scraper.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
