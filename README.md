# Museum Docs Generator

> Dynamic API Documentation Generator for Museum Digital Indonesia

Standalone service yang menghasilkan dokumentasi HTML interaktif dari YAML specification. Dirancang untuk AI agents dan frontend developers.

## Features

- **YAML-based** - Single source of truth, multi-file modular spec
- **Interactive UI** - ReactFlow architecture diagrams, flow steps, guide panels
- **API Tester** - Test endpoints langsung dari browser (JWT & API Key auth)
- **AI-ready** - JSON/YAML endpoint untuk AI agent consumption
- **Hot-reload** - Auto-refresh saat development mode
- **Multi-environment** - Support multiple base URLs (production, staging, dll)
- **Screen Documentation** - Dokumentasi screen/frontend pages

## Quick Start

```bash
# Clone
git clone git@github.com:Go-Routine-App/go-docs-generator.git
cd docs-generator

# Dependencies
go mod tidy

# Run (default: spec/index.yaml, port 8080)
go run cmd/server/main.go

# Development mode (hot-reload)
go run cmd/server/main.go -dev

# Custom spec & port
go run cmd/server/main.go -spec ./spec/index.yaml -port 9090
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/docs` | HTML Documentation |
| `/docs?p=<project>` | Project-specific docs |
| `/api/docs/spec` | API Spec as JSON (for AI) |
| `/api/docs/specs` | List available projects |
| `/api/docs/yaml` | Download raw YAML |
| `/api/docs/echo` | Debug - echo request headers |
| `/health` | Health check |

## Project Structure

```
docs-generator/
├── cmd/server/main.go       # Entry point
├── pkg/docs/
│   ├── types.go             # YAML struct definitions
│   ├── handler.go           # HTTP handlers
│   ├── loader.go            # Multi-file YAML loader
│   └── template.go          # HTML template (embedded)
├── spec/
│   ├── index.yaml           # Main spec (entry point)
│   ├── sections/            # Modular endpoint sections
│   │   ├── museum.yaml
│   │   ├── artifacts.yaml
│   │   └── articles.yaml
│   ├── guides/              # Guide documentation
│   │   └── file_upload.yaml
│   └── screens/             # Screen documentation
│       └── museum_screens.yaml
├── Dockerfile
├── Makefile
└── README.md
```

## Deployment with PM2

### Build & Start

```bash
cd /home/dev/museum/docs-generator

# Build binary
make build

# Start with PM2
pm2 start ./docs-generator --name docs-generator -- -spec ./spec/index.yaml -port 8080

# Save config (persist across reboots)
pm2 save
```

### Update & Redeploy

```bash
cd /home/dev/museum/docs-generator

# Pull latest code
git pull

# Rebuild
make build

# Restart
pm2 restart docs-generator

# Verify
pm2 logs docs-generator --lines 20
curl http://localhost:8080/health
```

### PM2 Auto-start on Reboot

```bash
# Generate startup script (run once)
pm2 startup systemd -u dev --hp /home/dev

# Save current process list
pm2 save

# Verify startup is configured
systemctl status pm2-dev
```

### PM2 Commands

```bash
pm2 status                     # List all processes
pm2 logs docs-generator        # View logs
pm2 restart docs-generator     # Restart
pm2 stop docs-generator        # Stop
pm2 delete docs-generator      # Remove from PM2
pm2 show docs-generator        # Show process details
```

## Nginx Configuration

Docs generator di-proxy melalui nginx:

```nginx
# Documentation page
location = /docs {
    proxy_pass http://127.0.0.1:8080/docs;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Docs API endpoints (spec, yaml, echo)
location /api/docs {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## Spec Format

Spec menggunakan format multi-file YAML. Entry point: `spec/index.yaml`.

Setiap file YAML di `spec/` directory (termasuk sub-directory) akan otomatis di-merge ke dalam spec utama.

Contoh `spec/index.yaml`:
```yaml
info:
  title: Museum Service API
  version: "1.0.0"
  base_url: https://museumdigi.id

authentication:
  methods:
    - type: Bearer JWT
      header: Authorization
      format: "Bearer <token>"
```

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-spec` | `./spec/index.yaml` | Path to spec file or directory |
| `-port` | `8080` | Server port |
| `-dev` | `false` | Enable hot-reload mode |

## For AI Agents

```bash
# Get JSON spec
curl https://museumdigi.id/api/docs/spec

# Download YAML
curl -O https://museumdigi.id/api/docs/yaml
```

## License

MIT License - Museum Digital Indonesia
