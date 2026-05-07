.PHONY: build run dev clean test generate docker-build docker-run install-skill uninstall-skill

# Build the binary
build:
	go build -o docs-generator cmd/server/main.go

# Regenerate JSON Schema + SPEC.md from Go structs and sync AGENTS.md into cmd/server
generate:
	go run ./cmd/gendocs
	cp AGENTS.md cmd/server/agents.md

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
	docker build -t docs-generator:latest .

# Run Docker container against ./spec
docker-run:
	docker run --rm -p 8080:8080 -v $(PWD)/spec:/app/spec docs-generator:latest

# Link the docs-gen-spec Claude skill into ~/.claude/skills/ so it auto-loads
# whenever the user works on a docs-generator spec — even from other projects.
# Symlink (not copy) so `git pull` keeps the skill up-to-date.
install-skill:
	@mkdir -p $(HOME)/.claude/skills
	@ln -sfn "$(CURDIR)/.claude/skills/docs-gen-spec" "$(HOME)/.claude/skills/docs-gen-spec"
	@echo "linked: $(HOME)/.claude/skills/docs-gen-spec -> $(CURDIR)/.claude/skills/docs-gen-spec"

uninstall-skill:
	@rm -f "$(HOME)/.claude/skills/docs-gen-spec"
	@echo "removed: $(HOME)/.claude/skills/docs-gen-spec"

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
	@echo "  install-skill - Link docs-gen-spec Claude Code skill globally"
	@echo "  uninstall-skill - Remove the linked skill"
