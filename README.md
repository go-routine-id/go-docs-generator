# 🏛️ Museum Docs Generator

> Dynamic API Documentation Generator for Museum Digital Indonesia

Standalone service that generates beautiful HTML documentation from YAML specification. Designed for AI agents and frontend developers.

## ✨ Features

- 📄 **YAML-based** - Single source of truth for API documentation
- 🎨 **Interactive UI** - Built with ReactFlow for architecture diagrams
- 🧪 **API Tester** - Test endpoints directly from the browser
- 🤖 **AI-ready** - JSON endpoint for AI agent consumption
- 🔄 **Hot-reload** - Auto-refresh in development mode

## 🚀 Quick Start

```bash
# Clone the repository
git clone <repo-url>
cd docs-generator

# Download dependencies
go mod tidy

# Run with default spec
go run cmd/server/main.go

# Or run with custom spec file
go run cmd/server/main.go -spec /path/to/your/api-spec.yaml

# Development mode (hot-reload)
go run cmd/server/main.go -dev
```

## 📚 Endpoints

| Endpoint | Description |
|----------|-------------|
| `/docs` | HTML Documentation with ReactFlow diagrams |
| `/api/docs/spec` | API Spec as JSON (for AI agents) |
| `/api/docs/yaml` | Download raw YAML file |
| `/health` | Health check |

## 🏗️ Project Structure

```
docs-generator/
├── cmd/server/         # Main application entry point
│   └── main.go
├── pkg/docs/           # Core documentation logic
│   ├── types.go        # YAML struct definitions
│   ├── handler.go      # HTTP handlers
│   └── template.go     # HTML template
├── api-spec.yaml       # Example API specification
├── go.mod
└── README.md
```

## 📝 API Spec Format

See `api-spec.yaml` for a complete example. The spec includes:

- **info** - API metadata (title, version, description)
- **authentication** - Auth configuration
- **sections** - Grouped endpoints with details
- **permissions** - Available permissions list
- **flow_diagram_nodes/edges** - ReactFlow diagram data
- **api_tester_defaults** - Default values for API tester

## 🔧 Configuration

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-spec` | `./api-spec.yaml` | Path to YAML spec file |
| `-port` | `8080` | Server port |
| `-dev` | `false` | Enable hot-reload mode |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GIN_MODE` | Set to `release` for production |

## 🐳 Docker

```bash
# Build image
docker build -t museum-docs .

# Run container
docker run -p 8080:8080 -v $(pwd)/api-spec.yaml:/app/api-spec.yaml museum-docs
```

## 🔗 Integration with Museum Service

This service is designed to be deployed alongside the Museum Service:

```
┌─────────────────┐     ┌─────────────────┐
│  Museum Service │     │  Docs Generator │
│    :9283        │     │    :8080        │
│  (Backend API)  │     │  (Documentation)│
└─────────────────┘     └─────────────────┘
         │                       │
         └──────────┬────────────┘
                    │
              ┌─────────┐
              │  Nginx  │
              │  :443   │
              └─────────┘
```

Nginx configuration:
```nginx
# Museum API
location /api/v1 {
    proxy_pass http://localhost:9283;
}

# Documentation
location /docs {
    proxy_pass http://localhost:8080;
}

location /api/docs {
    proxy_pass http://localhost:8080;
}
```

## 📦 Deployment

### Systemd Service

```ini
# /etc/systemd/system/museum-docs.service
[Unit]
Description=Museum Docs Generator
After=network.target

[Service]
Type=simple
User=dev
WorkingDirectory=/home/dev/museum/docs-generator
ExecStart=/home/dev/museum/docs-generator/docs-generator -spec ./api-spec.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable museum-docs
sudo systemctl start museum-docs
```

## 🤝 For AI Agents

AI agents can consume the API spec directly:

```bash
# Get JSON spec
curl https://museumdigi.id/api/docs/spec

# Or download YAML
curl -O https://museumdigi.id/api/docs/yaml
```

The JSON response contains complete API information including endpoints, permissions, and examples.

## 📝 License

MIT License - Museum Digital Indonesia
