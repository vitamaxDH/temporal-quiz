.PHONY: help serve pipeline worker test

# Convenience wrappers. The real targets live in worker/Makefile.
help:
	@echo "temporal-quiz - top-level commands"
	@echo "----------------------------------"
	@echo "  make serve     : preview the static UI at http://localhost:8080"
	@echo "  make pipeline  : scrape + generate + publish (delegates to worker/)"
	@echo "  make worker    : run the Temporal worker locally"
	@echo "  make test      : Go test suite"
	@echo ""
	@echo "See worker/Makefile for the full list of pipeline commands."

serve:
	@echo "Serving quiz UI at http://localhost:8080..."
	@cd docs && python3 -m http.server 8080

pipeline:
	@$(MAKE) -C worker --no-print-directory pipeline

worker:
	@$(MAKE) -C worker --no-print-directory worker

test:
	@$(MAKE) -C worker --no-print-directory test
