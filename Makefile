.PHONY: build run dev clean test generate docker-build docker-run

# Build the binary
build:
	go build -o docs-generator cmd/server/main.go

# Regenerate JSON Schema + SPEC.md from Go structs
generate:
	go run ./cmd/gendocs

# Run the server
run: build
	./docs-generator

# Run in development mode (hot-reload)
dev:
	go run cmd/server/main.go -dev

# Clean build artifacts
clean:
	rm -f docs-generator
	rm -f coverage.out

# Run tests
test:
	go test -v ./...

# Download dependencies
deps:
	go mod tidy
	go mod download

# Build Docker image
docker-build:
	docker build -t museum-docs:latest .

# Run Docker container
docker-run:
	docker run -p 8080:8080 -v $(PWD)/api-spec.yaml:/root/api-spec.yaml museum-docs:latest

# Install systemd service (requires sudo)
install-systemd:
	sudo cp museum-docs.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable museum-docs
	sudo systemctl start museum-docs

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  run           - Build and run the server"
	@echo "  dev           - Run in development mode (hot-reload)"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run tests"
	@echo "  generate      - Regenerate schemas/spec.schema.json and SPEC.md"
	@echo "  deps          - Download dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
