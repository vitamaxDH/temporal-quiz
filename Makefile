.PHONY: build test lint worker scrape quizgen localgen pipeline start-server stop-server clean wipe-all

help:
	@echo "Temporal Quiz - Available Commands:"
	@echo "------------------------------------"
	@echo "  make build        : Build all binaries (worker, starter, pipeline)"
	@echo "  make test         : Run all tests"
	@echo "  make lint         : Run go vet"
	@echo "  make worker       : Build and run the Temporal Worker"
	@echo "  make scrape       : Trigger the Scraper Workflow"
	@echo "  make quizgen      : Generate quizzes only (skip scraping)"
	@echo "  make localgen     : Read local/GitHub docs then generate quizzes"
	@echo "  make pipeline     : Run full daily pipeline (scrape + generate quizzes)"
	@echo "  make start-server : Start local Temporal Docker Cluster"
	@echo "  make stop-server  : Stop local Temporal Docker Cluster"
	@echo "  make clean        : Remove downloaded HTML files"
	@echo "  make wipe-all     : Remove ALL downloaded files and binaries"

build:
	@echo "Building binaries..."
	@go build -o bin/worker ./cmd/worker
	@go build -o bin/starter ./cmd/starter
	@go build -o bin/pipeline ./cmd/pipeline
	@go build -o bin/quizgen ./cmd/quizgen
	@go build -o bin/localgen ./cmd/localgen
	@echo "Built: bin/worker, bin/starter, bin/pipeline, bin/quizgen, bin/localgen"

test:
	@go test ./... -v

lint:
	@go vet ./...

worker: build
	@echo "Starting Temporal Worker... (Press Ctrl+C to stop)"
	@GODEBUG=netdns=cgo ./bin/worker

scrape: build
	@echo "Triggering Scraper Workflow..."
	@GODEBUG=netdns=cgo ./bin/starter

EASY ?= 3
MED ?= 4
HARD ?= 4
NIGHTMARE ?= 2

quizgen: build
	@echo "Generating quizzes (easy=$(EASY), med=$(MED), hard=$(HARD), nightmare=$(NIGHTMARE))..."
	@GODEBUG=netdns=cgo ./bin/quizgen -easy=$(EASY) -med=$(MED) -hard=$(HARD) -nightmare=$(NIGHTMARE)

DOCS_DIR ?=

localgen: build
	@echo "Reading docs and generating quizzes..."
	@GODEBUG=netdns=cgo ./bin/localgen $(if $(DOCS_DIR),-docs=$(DOCS_DIR)) -easy=$(EASY) -med=$(MED) -hard=$(HARD) -nightmare=$(NIGHTMARE)

pipeline: build
	@echo "Running full pipeline (scrape + generate quizzes)..."
	@GODEBUG=netdns=cgo ./bin/pipeline

start-server:
	@echo "Starting Temporal cluster via Docker Compose..."
	@cd temporal-docker-compose && docker-compose up -d
	@echo "Temporal Web UI available at: http://localhost:8081"

stop-server:
	@echo "Stopping Temporal cluster..."
	@cd temporal-docker-compose && docker-compose down

clean:
	@echo "Cleaning up raw HTML files..."
	@rm -rf temporal_docs_html

wipe-all: clean
	@echo "Wiping all processed txt files and binaries..."
	@rm -rf temporal_docs_txt bin/
