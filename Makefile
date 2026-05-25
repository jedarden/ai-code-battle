# AI Code Battle Makefile

.PHONY: all build test clean map-library

# Default target
all: build

# Build all binaries
build:
	go build ./...

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run engine tests only
test-engine:
	go test -v ./engine/...

# Build acb-local binary
acb-local:
	go build -o bin/acb-local ./cmd/acb-local

# Build acb-mapgen binary
acb-mapgen:
	go build -o bin/acb-mapgen ./cmd/acb-mapgen

# Generate the map library (50 maps per player count)
map-library:
	@echo "Generating map library..."
	@./scripts/generate-map-library.sh

# Clean build artifacts
clean:
	go clean ./...
	rm -rf bin/
