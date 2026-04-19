.PHONY: serve

serve:
	@echo "Serving quiz UI at http://localhost:8080..."
	@cd web && python3 -m http.server 8080
