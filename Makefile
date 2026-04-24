.PHONY: up down tui clean logs

# Start all backend services in detached mode
up:
	@echo "Starting ATARE Distributed Infrastructure..."
	docker-compose up -d --build
	@echo "Services are spinning up. Wait ~30 seconds for Redpanda and Ollama to initialize."

# Stop and remove containers, networks
down:
	@echo "Tearing down ATARE infrastructure..."
	docker-compose down

# Read the streaming logs of the processor and agentic engine
logs:
	docker-compose logs -f stream_processor agentic_engine

# Run the TUI Dashboard locally
tui:
	@echo "Starting the ATARE Terminal UI..."
	cd tui_dashboard && pip install -r requirements.txt && python app.py

# Clean persistent volumes
clean: down
	@echo "Wiping local state..."
	rm -rf shared_data/*.db
	docker volume rm projectalpha_ollama_data || true
