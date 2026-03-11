APP_NAME = orchestrator
MAIN_PKG = ./cmd/server

.PHONY: all build run test clean docker-build docker-run lint auth-gcloud setup-go-mod

all: build

build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) $(MAIN_PKG)

run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_PKG)/main.go

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning up..."
	rm -rf bin/

docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):latest .

docker-run:
	@echo "Running Docker container..."
	@if [ -z "$(GEMINI_API_KEY)" ]; then \
		echo "GEMINI_API_KEY is not set. Please set it to run the container."; \
		exit 1; \
	fi
	docker run -e GEMINI_API_KEY=$(GEMINI_API_KEY) --rm $(APP_NAME):latest

lint:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Vetting code..."
	go vet ./...

setup-go-mod:
	@echo "Downloading modules..."
	go mod tidy
