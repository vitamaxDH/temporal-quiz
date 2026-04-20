.PHONY: help worker scrape quizgen localgen pipeline publish start-server stop-server clean test build

# Per-difficulty question counts (used by quizgen + localgen)
EASY ?= 3
MED ?= 4
HARD ?= 4
NIGHTMARE ?= 2
DOCS_DIR ?=

# Publish targets
UI_REPO ?= ../temporal-quiz-ui
UI_QUIZZES = $(UI_REPO)/docs/quizzes
SRC_QUIZZES = web/quizzes
TODAY = $(shell date -u +%Y-%m-%d)

help:
	@echo "Temporal Quiz - Available Commands"
	@echo "----------------------------------"
	@echo ""
	@echo "Individual steps:"
	@echo "  make worker       : Build and run the Temporal Worker (long-running)"
	@echo "  make scrape       : Trigger the Scraper Workflow (pulls latest docs)"
	@echo "  make quizgen      : Generate quizzes from already-scraped docs"
	@echo "  make localgen     : Read local/GitHub docs then generate quizzes"
	@echo ""
	@echo "End-to-end:"
	@echo "  make pipeline     : scrape + generate quizzes + publish to UI repo"
	@echo "  make publish      : Publish today's run to UI repo (standalone)"
	@echo "                       (override UI_REPO=path/to/temporal-quiz-ui)"
	@echo ""
	@echo "Infrastructure:"
	@echo "  make start-server : Start local Temporal Docker Cluster"
	@echo "  make stop-server  : Stop local Temporal Docker Cluster"
	@echo ""
	@echo "Utility:"
	@echo "  make test         : Run the Go test suite"
	@echo "  make clean        : Remove scraped docs + binaries"

# --- Build (implicit dependency of individual targets, not shown in help)
build:
	@go build -o bin/worker ./cmd/worker
	@go build -o bin/starter ./cmd/starter
	@go build -o bin/pipeline ./cmd/pipeline
	@go build -o bin/quizgen ./cmd/quizgen
	@go build -o bin/localgen ./cmd/localgen

test:
	@go test ./... -v

# --- Individual steps
worker: build
	@echo "Starting Temporal Worker... (Press Ctrl+C to stop)"
	@GODEBUG=netdns=cgo ./bin/worker

scrape: build
	@echo "Triggering Scraper Workflow..."
	@GODEBUG=netdns=cgo ./bin/starter

quizgen: build
	@echo "Generating quizzes (easy=$(EASY), med=$(MED), hard=$(HARD), nightmare=$(NIGHTMARE))..."
	@GODEBUG=netdns=cgo ./bin/quizgen -easy=$(EASY) -med=$(MED) -hard=$(HARD) -nightmare=$(NIGHTMARE)

localgen: build
	@echo "Reading docs and generating quizzes..."
	@GODEBUG=netdns=cgo ./bin/localgen $(if $(DOCS_DIR),-docs=$(DOCS_DIR)) -easy=$(EASY) -med=$(MED) -hard=$(HARD) -nightmare=$(NIGHTMARE)

# --- Full pipeline: scrape + generate + publish
pipeline: build
	@echo "Running full pipeline (scrape + generate quizzes)..."
	@GODEBUG=netdns=cgo ./bin/pipeline
	@$(MAKE) --no-print-directory publish

# --- Publish today's run into the UI repo.
#
# Layout of web/quizzes/ after a pipeline run:
#   web/quizzes/runs/<YYYY-MM-DD>/...    (today's snapshot)
#
# This target mirrors the dated run dir into $(UI_REPO)/docs/quizzes/runs/,
# regenerates runs.json from whatever date dirs exist in the destination,
# then commits and pushes. Override UI_REPO=... if your UI checkout lives
# elsewhere.
publish:
	@if [ ! -d "$(SRC_QUIZZES)/runs/$(TODAY)" ]; then \
	  echo "error: $(SRC_QUIZZES)/runs/$(TODAY) does not exist. Run 'make pipeline' or 'make quizgen' first."; \
	  exit 1; \
	fi
	@if [ ! -d "$(UI_REPO)/.git" ]; then \
	  echo "error: $(UI_REPO) is not a git repo. Set UI_REPO=path/to/temporal-quiz-ui."; \
	  exit 1; \
	fi
	@echo "Publishing quizzes to $(UI_QUIZZES)/runs/$(TODAY)..."
	@mkdir -p "$(UI_QUIZZES)/runs/$(TODAY)"
	@cp $(SRC_QUIZZES)/runs/$(TODAY)/*.json "$(UI_QUIZZES)/runs/$(TODAY)/"
	@python3 scripts/build_runs_index.py "$(UI_QUIZZES)"
	@cd "$(UI_REPO)" && git add docs/quizzes && \
	  if git diff --cached --quiet; then \
	    echo "No changes to commit."; \
	  else \
	    git commit -m "data: refresh quizzes $(TODAY)" && git push; \
	  fi

# --- Infrastructure
start-server:
	@echo "Starting Temporal cluster via Docker Compose..."
	@cd temporal-docker-compose && docker-compose up -d
	@echo "Temporal Web UI available at: http://localhost:8081"

stop-server:
	@echo "Stopping Temporal cluster..."
	@cd temporal-docker-compose && docker-compose down

# --- Maintenance
clean:
	@echo "Cleaning up scraped docs + binaries..."
	@rm -rf temporal_docs_html temporal_docs_txt bin/
