.PHONY: help up down lint test

help:
	@echo "Available targets:"
	@echo "  up     - Start Docker services (Chroma, Ollama, Temporal)"
	@echo "  down   - Stop Docker services"
	@echo "  lint   - Run linters on Python labs"
	@echo "  test   - Run Python tests"

up:
	docker compose up -d

down:
	docker compose down

lint:
	cd labs && find . -path '*/python/*.py' -not -path '*/venv/*' -not -path '*/.venv/*' -exec python -m py_compile {} \;

test:
	@echo "Run individual lab READMEs; no global test suite yet."
