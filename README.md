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


## Screenshoots

<img width="1800" height="1126" alt="image" src="https://github.com/user-attachments/assets/e8f6c882-dc5f-4a83-b70b-d02acc69e6a5" />
<img width="1800" height="1126" alt="image" src="https://github.com/user-attachments/assets/203f0030-6fd3-48de-9514-44a53a99fc6e" />
<img width="1800" height="1126" alt="image" src="https://github.com/user-attachments/assets/5b1dc197-f4fc-457e-8df9-b266ab9fd5b3" />
<img width="1800" height="1126" alt="image" src="https://github.com/user-attachments/assets/0119c12b-956f-450b-b679-4f81fcc3ab8d" />



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

## For AI coding agents

If you are an AI assistant (Claude Code, Cursor, Copilot, Cline, …) working in **someone else's project** and want to generate a docs-generator spec for that project, read [`AGENTS.md`](AGENTS.md). It is self-contained, covers monolith and microservice patterns, common pitfalls, and a self-check list.

Ways to access it:

```bash
# Option 1 — if docs-gen is installed, pipe the embedded copy:
docs-gen prompt

# Option 2 — if docs-generator is running somewhere reachable, fetch over HTTP:
curl https://your-docs-host/docs/agents

# Option 3 — fetch directly from GitHub (stable raw URL):
curl https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/AGENTS.md
```

### Claude Code skill (recommended for Claude users)

If you use Claude Code, this repo ships a skill at [`.claude/skills/docs-gen-spec/`](.claude/skills/docs-gen-spec/SKILL.md) that auto-loads the authoring rules whenever you work on a docs-generator spec — no need to remember to fetch `AGENTS.md` first. The skill triggers on phrases like "edit api-spec.yaml", "tester crashes", "auth_modes", and similar.

**Install once per machine** (clone this repo, then run from its root):

```bash
make install-skill
```

That symlinks `.claude/skills/docs-gen-spec/` into `~/.claude/skills/`, so the skill is active in **every** Claude Code session — including when you are working in another project that consumes docs-generator. `git pull` automatically picks up updates because it is a symlink, not a copy.

To verify the link, open a new Claude Code session anywhere and ask it to edit any `api-spec.yaml`; it should run `docs-gen prompt` (or fetch `/docs/agents`) before touching the file. To remove:

```bash
make uninstall-skill
```

If you cannot use `make`, the same effect:

```bash
ln -sfn "$PWD/.claude/skills/docs-gen-spec" ~/.claude/skills/docs-gen-spec
```

## License

MIT
