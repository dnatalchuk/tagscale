# Makefile
.PHONY: build build-cli run test clean docker-build docker-run

# Build the application
build:
	go build -o bin/tagscale ./cmd/main.go

# Build the CLI application
build-cli:
	go build -o bin/tagscale-cli ./cmd/cli

# Run the application
run:
	go run ./cmd/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod tidy
	go mod download

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Docker build
docker-build:
	docker build -t tagscale:latest .

# Docker run
docker-run:
	docker-compose up -d

# Docker stop
docker-stop:
	docker-compose down

# Database migration
migrate:
	go run ./cmd/migrate/main.go

# Run development server
dev:
	air

# Generate API documentation
docs:
	swag init -g ./cmd/main.go
