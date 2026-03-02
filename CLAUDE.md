# Museum Docs Generator - Claude Context

## Project Overview

This is a standalone documentation generator service for Museum Digital Indonesia.
It generates dynamic HTML documentation from YAML API specifications.

**Repository:** `git@github.com:Go-Routine-App/go-docs-generator.git`
**Location:** `/home/dev/museum/docs-generator/`

## Quick Commands

```bash
# Development (hot-reload mode)
go run cmd/server/main.go -dev

# Build
make build

# Run production
./docs-generator -spec ./api-spec.yaml -port 8080

# Docker
make docker-build
make docker-run
```

## Project Structure

```
docs-generator/
├── cmd/server/main.go      # Entry point
├── pkg/docs/
│   ├── types.go            # YAML struct definitions
│   ├── handler.go          # HTTP handlers with hot-reload
│   └── template.go         # HTML template (embedded)
├── api-spec.yaml           # API spec (source of truth)
├── Dockerfile
├── Makefile
└── museum-docs.service     # Systemd service file
```

## Key Features

1. **Dynamic HTML Generation** - Renders docs from YAML on each request (dev mode)
2. **ReactFlow Diagrams** - Interactive service architecture visualization
3. **API Tester** - In-browser API testing with JWT token storage
4. **AI-Ready Endpoints:**
   - `/api/docs/spec` - JSON format for AI agents
   - `/api/docs/yaml` - Raw YAML download

## Integration

This service runs standalone and is proxied by nginx:

```nginx
location /docs {
    proxy_pass http://localhost:8080;
}
location /api/docs {
    proxy_pass http://localhost:8080;
}
```

## Environment

- **Go version:** 1.21+
- **Default port:** 8080
- **Dev mode:** `-dev` flag enables hot-reload

## Notes for Claude

- YAML spec is the single source of truth
- Template uses Go's `html/template` with custom functions
- File watcher polls every 2 seconds in dev mode
- No external database - pure file-based
