# Variables
APP_NAME := myapp
SRC := $(wildcard *.go)
DB_CONTAINER := learn-go-auth-db-1

# Default target: build the application
all: build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) $(SRC)

# Run the application (builds first)
run: build
	@echo "Running $(APP_NAME)..."
	./$(APP_NAME)

# Clean up the binary
clean:
	@echo "Cleaning up..."
	rm -f $(APP_NAME)

# Check the database container status
check-db:
	@echo "Checking PostgreSQL container status..."
	docker ps | grep $(DB_CONTAINER)

# Rebuild the application
rebuild: clean build

# Run the application with environment variables loaded from .env
run-env: build
	@echo "Loading environment variables from .env..."
	export $$(grep -v '^#' .env | xargs) && ./$(APP_NAME)

# Help command to show available targets
help:
	@echo "Available targets:"
	@echo "  build        - Build the Go application"
	@echo "  run          - Build and run the application"
	@echo "  clean        - Remove the built binary"
	@echo "  check-db     - Check if the PostgreSQL container is running"
	@echo "  rebuild      - Clean and build the application"
	@echo "  run-env      - Run the application with environment variables loaded from .env"
	@echo "  help         - Show this help message"
