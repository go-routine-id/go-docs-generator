# docs-generator

Interactive API documentation generator — point it at a YAML spec, get a single-page site with live tester, architecture diagrams, flows, and screens.

Works for **monoliths and microservices**. Per-section base URLs let one documentation page describe many backends (e.g. `account-service` and `storage-service` with different hosts). Accepts both a native YAML format and OpenAPI 3.x.

## Features

- **YAML-based** — single source of truth, multi-file merging, no build step.
- **OpenAPI 3.x compatible** — point at your existing `swagger.yaml` and it just works.
- **Interactive API tester** — send requests from the browser, per-environment credentials in `localStorage`.
- **Architecture diagrams** — ReactFlow nodes & edges; auto-layout via dagre when positions aren't set.
- **Per-section base URL** — mix services with different backends on one page.
- **Events/async docs** — document Kafka topics, webhooks, and pub/sub channels alongside HTTP endpoints.
- **Theming** — override title, logo, primary color, favicon without touching the template.
- **Hot reload** in dev mode.
- **AI-ready endpoints** — `/docs/spec` (JSON), `/docs/openapi` (OpenAPI), `/docs/yaml` (raw).

## Quick start

```bash
git clone <this-repo>
cd docs-generator
go mod tidy

# Scaffold a minimal spec
go run ./cmd/server init myspec

# Serve it (defaults: port 8080, prefix /docs)
go run ./cmd/server -spec ./myspec/index.yaml

# Open http://localhost:8080/docs
```

Prefer your existing OpenAPI file?

```bash
go run ./cmd/server -spec ./swagger.yaml
```

The server auto-detects OpenAPI documents and projects them onto the internal model.

## CLI

```
docs-gen                       start the HTTP server (default)
docs-gen serve [flags...]      explicit server mode
docs-gen validate <path>       verify a spec, exit 1 on failure (CI-friendly)
docs-gen init [dir]            scaffold a minimal spec/ directory
```

### Server flags

| Flag | Default | Description |
|------|---------|-------------|
| `-spec` | `./spec/index.yaml` | File or directory. Directories are scanned for `index.yaml` and auto-merged. |
| `-port` | `8080` | HTTP port. |
| `-prefix` | `/docs` | URL prefix (route under a reverse proxy). |
| `-dev` | `false` | Hot-reload when spec files change. |
| `-log-format` | auto | `text` or `json` (auto-picks JSON when `GIN_MODE=release`). |

## HTTP endpoints

| Path | Purpose |
|------|---------|
| `/docs` | HTML documentation |
| `/docs?p=<project>` | Specific project (multi-project mode) |
| `/docs/spec` | Internal spec as JSON (for AI agents and tooling) |
| `/docs/specs` | List available projects |
| `/docs/yaml` | Raw YAML download |
| `/docs/openapi` | Export as OpenAPI 3.0 JSON |
| `/docs/echo` | Debug: echo request headers |
| `/health` | Health check |

## Spec format

Three ways to learn the spec, ordered by depth:

1. **[docs/writing-specs.md](docs/writing-specs.md)** — narrative guide with worked examples (monolith vs microservice, merge rules, conventions, FAQ). Start here.
2. **[SPEC.md](SPEC.md)** — auto-generated field reference (every field, every type).
3. **[schemas/spec.schema.json](schemas/spec.schema.json)** — machine-readable JSON Schema for IDE autocomplete.

Reference the schema from your YAML for autocomplete + lint:

```yaml
# yaml-language-server: $schema=./schemas/spec.schema.json
```

### Multi-file specs

Point `-spec` at a directory and every `.yaml` file inside is merged:

```
spec/
├── index.yaml          # global info + authentication
├── sections/
│   ├── account.yaml    # sections: [ ... ]
│   └── storage.yaml    # sections: [ ... ]
└── guides/
    └── upload.yaml     # guides: [ ... ]
```

Merge rules:

- **Slices** are appended (every file contributes).
- **Nested objects** are merged per-field (overlay non-zero values win).
- **Scalars** are overridden when overlay is non-zero.

### Multi-project mode

Subdirectories with their own `index.yaml` become separate projects accessible at `/docs?p=<name>`:

```
projects/
├── index.yaml              # default project
├── account/index.yaml      # /docs?p=account
└── storage/index.yaml      # /docs?p=storage
```

### Per-section base URL

Document several services in one page — each section can override the document-level base URL:

```yaml
sections:
  - id: account
    title: Account Service
    base_url: https://account.example.com
    base_urls:
      - label: Prod
        url: https://account.example.com
        default: true
      - label: Staging
        url: https://staging.account.example.com
    endpoints:
      - name: Login
        method: POST
        path: /v1/login
        description: ...
  - id: storage
    title: Storage Service
    base_url: https://storage.example.com
    endpoints:
      - name: Upload
        method: POST
        path: /v1/files
        description: ...
```

Sections without their own `base_url(s)` inherit from `info.base_urls` and follow the global environment selector.

## Examples

- [`examples/museum/`](examples/museum/) — full-featured spec with sections, guides, screens, flow diagrams, and multi-environment base URLs.
- [`spec/`](spec/) — minimal starter.

## Deployment

### Build

```bash
make build
./docs-generator -spec ./spec/index.yaml -port 8080
```

### Docker

```bash
make docker-build
docker run -p 8080:8080 \
  -v $(pwd)/spec:/spec \
  docs-generator:latest -spec /spec/index.yaml
```

### Reverse proxy (nginx)

```nginx
location = /docs       { proxy_pass http://127.0.0.1:8080/docs; }
location  /docs/       { proxy_pass http://127.0.0.1:8080; }
```

## Development

```bash
make dev              # hot-reload server
make test             # run all tests
make generate         # regenerate SPEC.md and schemas/spec.schema.json
```

## License

MIT
