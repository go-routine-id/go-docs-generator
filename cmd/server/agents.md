# AGENTS.md — Instructions for AI Coding Agents

> **Audience:** AI coding assistants (Claude Code, Cursor, Copilot, Cline, etc.) working in a user's codebase who need to **author a `docs-generator` spec** for that project.
>
> **This file is self-contained.** You do not need to fetch other files to generate a valid spec. Stable URLs for deeper reference are listed at the end.

## What docs-generator is

An HTTP server that renders an interactive API documentation page from a YAML specification. One repo can document either a monolith or multiple microservices. The server also accepts OpenAPI 3.x as input.

Upstream repo: `github.com/Go-Routine-App/go-docs-generator`

## Your task

When the user asks to "document our API using docs-generator" or similar, produce a YAML spec file (or a directory of YAML files) that:

1. Validates against the schema below.
2. Reflects the actual endpoints of the user's project — derive from source code, not from imagination.
3. Uses the patterns in this file (single-file for small APIs, multi-file for larger ones, per-section base URL for microservices).

## Authoring workflow (follow this order)

1. **Survey the codebase.** Find route handlers, controllers, RPC definitions. Extract method + path + auth expectation + request/response shape.
2. **Decide mode** (see §"Modes"). Default to single-file unless the project has > ~5 logical groups or > ~300 lines.
3. **Write `spec/index.yaml`** following §"Required structure". Start minimal — `info` + `sections` with endpoints. Add `guides`, `screens`, `events`, `theme` only if the project actually has those concerns.
4. **Reference the schema** by adding this on line 1 of each YAML file:
   ```yaml
   # yaml-language-server: $schema=https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/schemas/spec.schema.json
   ```
5. **Validate.** If the user has the `docs-gen` binary installed:
   ```bash
   docs-gen validate ./spec/index.yaml
   ```
   Exit 0 = valid. Otherwise, fix the reported error and revalidate.
6. **Serve (optional).**
   ```bash
   docs-gen -spec ./spec/index.yaml
   # Open http://localhost:8080/docs
   ```

---

## Modes

| Mode | When | `-spec` arg |
|------|------|-------------|
| Single-file | < 5 logical groups, < 300 lines | `./api.yaml` |
| Multi-file directory | medium API, multiple contributors | `./spec/index.yaml` |
| Multi-project | dev portal hosting several independent services | `./projects/` |

### Multi-file directory layout

```
spec/
├── index.yaml            # info, authentication, theme, api_tester_defaults
├── sections/             # one file per logical group
│   ├── users.yaml
│   └── orders.yaml
├── guides/               # optional — multi-endpoint flows
│   └── checkout.yaml
├── screens/              # optional — frontend page ↔ API mapping
│   └── dashboard.yaml
└── events/               # optional — you may also put events: in index.yaml
    └── webhooks.yaml
```

All `.yaml`/`.yml` under the directory are merged. Rules:
- **Slice fields** (sections, guides, screens, events, permissions, …) are **appended** — every file contributes.
- **Nested object fields** (`info`, `authentication`, …) are **merged per-field** — overlay non-zero values override.
- **Scalar fields** — overlay wins when non-zero. Zero values (`""`, `0`, `false`, `null`) do **not** clobber the base.

### Multi-project layout

```
projects/
├── index.yaml              # default project (shown at /docs)
├── account/index.yaml      # shown at /docs?p=account
└── storage/index.yaml      # shown at /docs?p=storage
```

Only subdirectories containing `index.yaml` become projects.

---

## Required structure

Minimum viable spec:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/schemas/spec.schema.json
info:
  title: <API Name>
  version: <semver or date string>
  description: <one-paragraph summary>
  base_urls:
    - label: Production
      url: https://api.example.com
      default: true

sections:
  - id: <kebab-case-id>
    title: <Human-readable>
    description: <what this group covers>
    endpoints:
      - name: <Action name>
        method: GET|POST|PATCH|PUT|DELETE
        path: /v1/resource
        auth: none|JWT|API Key|...
        description: <what it does>
```

All other top-level fields (`authentication`, `guides`, `screens`, `events`, `theme`, `permissions`, `constraints`, `flow_diagram_*`, `api_tester_defaults`) are optional — only include them if they add real information.

---

## Top-level fields (cheat sheet)

```yaml
info:
  title: string                   # required in practice
  version: string
  description: string             # markdown-lite supported
  base_url: string                # fallback for tester URL input
  base_urls:                      # env dropdown
    - { label: string, url: string, default: bool }
  overview_cards:                 # hero cards on overview page
    - { icon, title, description, content }  # content is markdown

authentication:
  methods:
    - type: Bearer JWT | API Key | Basic | OAuth2 | ...
      header: Authorization | X-API-Key | ...
      format: "Bearer <token>" | "<api_key>" | ...
      description: string
      source: string              # which service issues it
      note: string
      token_contains: [string]    # claims a token carries

sections:
  - id: users                     # kebab-case, unique
    title: Users
    description: string
    base_url: string              # optional — PER-SECTION override
    base_urls:                    # optional — section-specific env dropdown
      - { label, url, default }
    endpoints:
      - name: string
        method: GET|POST|...
        path: /v1/...
        auth: string              # free-form label, match across spec
        permission: string        # optional — shown as badge
        description: string
        query_params:
          - { name, type, required, default, description }
        body:
          - { name, type, required, description, example }
        example_body: |           # raw string, multi-line OK
          { "x": 1 }
        example_response: |
          { "ok": true }

guides:                           # multi-endpoint flows
  - id: file_upload
    icon: "📤"
    title: File Upload
    description: string
    flow:
      - step: 1
        title: Upload
        description: string
        endpoint:                 # inline endpoint detail
          method: POST
          path: /upload
          service: media-service
          content_type: multipart/form-data
          auth: JWT
          permission: media:upload
          fields:
            - { name, type, required, description }
        curl_example: |
          curl ...
        curl_example_jwt: |       # variant for JWT auth
          curl ...
        curl_example_api_key: |   # variant for API Key auth
          curl ...
        response_example: |
          { ... }
        actions:                  # links to other parts of the doc
          - { type: link, description, endpoint: "#anchor-id" }

screens:                          # frontend/mobile pages + their API calls
  - id: dashboard
    icon: "📊"
    title: Dashboard
    description: string
    image: /path/to/screenshot.png    # optional
    platform: [Web, Mobile]
    calls:
      - method: GET
        path: /v1/me
        purpose: string
        trigger: "On mount" | "On click 'Save'" | ...
        auth: required | optional | none
        notes: string

events:                           # async channels
  - id: user-signup
    title: User Signup
    description: string
    protocol: kafka | amqp | mqtt | nats | webhook | sse | websocket
    address: topic-name | queue-name | URL
    operations:
      - type: publish | subscribe  # from documented service's perspective
        summary: string
        description: string
        payload:
          - { name, type, required, description, example }
        example: |
          { ... }

permissions:                      # permission dictionary
  - { name: "users:read", description: "..." }

constraints:                      # rules/invariants shown on overview
  - "One tenant = one workspace"
  - "File uploads must go through /upload"

flow_diagram_nodes:               # architecture diagram (ReactFlow)
  - id: auth
    label: "🔐 Auth Service"
    type: service | data | artifact | article | museum | ...
    color: "#4f46e5"
    position: { x: 100, y: 50 }   # if ALL nodes have (0,0), dagre auto-layouts

flow_diagram_edges:
  - source: auth
    target: gateway
    label: string
    animated: bool
    color: "#4f46e5"
    style: dashed                 # empty = solid

api_tester_defaults:
  methods: [GET, POST, PATCH, DELETE, PUT]
  auth_modes:
    - name: JWT Bearer
      header: Authorization
      prefix: "Bearer "
      placeholder: YOUR_JWT_TOKEN_HERE

theme:                            # branding — all fields optional
  title: string                   # overrides Info.Title in UI
  logo_icon: string               # emoji or short string
  logo_image: /path/to/logo.svg   # overrides logo_icon
  primary_color: "#ff6600"
  favicon: /favicon.ico
```

---

## Common patterns

### Pattern A — Monolith (single backend)

All endpoints share one base URL. Skip per-section overrides:

```yaml
info:
  title: My App API
  version: "1.0"
  base_urls:
    - { label: Prod, url: https://api.myapp.com, default: true }
    - { label: Staging, url: https://staging.myapp.com }
sections:
  - id: users
    title: Users
    endpoints: [...]
  - id: orders
    title: Orders
    endpoints: [...]
```

### Pattern B — Microservices (one docs page, many services)

Each section = one service, each with its own base URL:

```yaml
info:
  title: Platform API
sections:
  - id: account
    title: Account Service
    base_url: https://account.example.com
    base_urls:
      - { label: Prod, url: https://account.example.com, default: true }
      - { label: Staging, url: https://staging.account.example.com }
    endpoints: [...]

  - id: storage
    title: Storage Service
    base_url: https://storage.example.com
    endpoints: [...]

  - id: billing
    title: Billing Service
    base_url: https://billing.example.com
    endpoints: [...]
```

The global environment dropdown only affects sections **without** their own `base_url(s)`.

### Pattern C — Microservices with independent docs per service

When each service has its own team and release cadence, use multi-project mode — one subdirectory per service:

```
dev-portal/
├── index.yaml                   # landing/overview (can be minimal)
├── account/
│   ├── index.yaml
│   └── sections/*.yaml
└── storage/
    ├── index.yaml
    └── sections/*.yaml
```

Run: `docs-gen -spec ./dev-portal/`.

### Pattern D — Existing OpenAPI

If the project already has `openapi.yaml` or `swagger.json`, you **don't** need to rewrite it in docs-generator's format. Point `-spec` at the file directly:

```bash
docs-gen -spec ./openapi.yaml
```

The server auto-detects (by `openapi:` key) and projects it onto the internal model. You may still *add* a sibling YAML with docs-generator-specific extras (guides, screens, events, theme). In that case use multi-file mode with the OpenAPI as one of the files.

---

## Conventions

1. **`id` fields** must be lowercase, URL-friendly (kebab-case or snake_case, consistent within a project). Used as HTML anchors.
2. **`auth` label** is free-form string — pick a convention and stick to it (`JWT`, `API Key`, `none`, `JWT | API Key`). Don't mix `required` and `JWT` in the same spec.
3. **Multi-line strings** — use YAML block scalar `|` to preserve newlines (for `example_body`, `overview_cards[].content`, `curl_example`, etc.).
4. **Descriptions** support simple Markdown: `**bold**`, `*italic*`, `` `code` ``, `- list`, `# heading` (renders as h3).
5. **Never duplicate `id`** within the same type across overlay files — the loader appends blindly, you'll see ghost duplicates.
6. **Placeholder URLs** in `example_body` / `curl_example` — use the base URL placeholder notation if you want, but the tester will prepend the selected `base_url` automatically, so keep example bodies focused on the payload.

---

## Validation

Three access paths, same two layers underneath (JSON Schema + semantic linter). Pick whichever is easiest in your environment.

### Option 1 — HTTP endpoint (zero install, best for sandboxed AI agents)

If *any* docs-generator instance is reachable from your environment, POST the spec body and get a JSON response:

```bash
curl -sf -X POST https://<docs-host>/docs/validate \
  -H 'Content-Type: text/yaml' \
  --data-binary @./spec/index.yaml
```

Response shape (stable, `snake_case`):

```json
{
  "ok": true,
  "schema_errors": [],
  "diagnostics": [
    { "severity": "warning", "path": ".sections[0]", "message": "section has no description" }
  ],
  "summary": { "schema_errors": 0, "lint_errors": 0, "lint_warnings": 1 }
}
```

- `ok` is `true` only when `schema_errors` is empty AND no diagnostic has `severity: "error"`. Warnings do not flip `ok`.
- Body accepted: YAML (`Content-Type: text/yaml`, `application/yaml`, `application/x-yaml`, `text/plain`) or JSON (`application/json`). Max 1 MiB.
- Endpoint is idempotent and carries no side effects — safe to call from AI loops.

**Multi-file specs:** concatenate all files into a single merged YAML before posting (e.g. `cat spec/**/*.yaml`). The endpoint validates one document at a time; it doesn't do directory merge for you.

### Option 2 — CLI (best when a local binary is installed)

```bash
# Level 1 — parse, multi-file merge, and JSON Schema conformance
docs-gen validate ./spec/index.yaml
# → "ok: ..." (exit 0) or "invalid: ..." (exit 1)

# Level 2 — semantic checks (dup ids, dangling anchors, orphan permission
# references, inconsistent auth labels, empty descriptions)
docs-gen lint ./spec/index.yaml
# → "clean: ..." or lists "✖ error" / "⚠ warning" lines
# → exit 0 on warnings, exit 1 on errors; use `-strict` to fail on warnings
```

### Option 3 — CI

```yaml
- name: Validate + lint spec
  run: |
    go run github.com/Go-Routine-App/go-docs-generator/cmd/server@latest validate ./spec/index.yaml
    go run github.com/Go-Routine-App/go-docs-generator/cmd/server@latest lint -strict ./spec/index.yaml
```

### Fallback — pure JSON Schema validation

No binary, no reachable server? Use any Draft 2020-12 validator (`ajv`, Python `jsonschema`, `jv`, …) against `schemas/spec.schema.json` after converting YAML → JSON. This gives you Level 1 only — semantic checks (duplicate ids, dangling anchors) still need Option 1 or 2.

---

## Anti-patterns (do NOT do these)

- ❌ Inventing endpoints the codebase doesn't have. Always derive from source.
- ❌ Hardcoding production URLs in `example_body` / response examples when `base_urls` exist.
- ❌ Putting secrets (tokens, keys) in `example_body` or `example_response`.
- ❌ Defining the same `section.id` in two overlay files.
- ❌ Using overlay empty-string values expecting them to erase base values — zero values are ignored by the merger.
- ❌ Skipping `description` fields — a docs page with only names is not useful.
- ❌ Manually positioning 50+ flow diagram nodes — leave `position` unset and let dagre auto-layout.
- ❌ Leaving a literal `:` followed by a space inside an **unquoted** scalar value. `title: Flow: Render Model` fails YAML parsing ("mapping values are not allowed in this context"). Wrap the whole value in double quotes: `title: "Flow: Render Model"`. Same applies to `description`, `summary`, any string field.

---

## Stable URLs for deeper reference

If you need more context beyond this file, fetch these (they are versioned alongside the tool):

| Resource | URL | What's there |
|----------|-----|--------------|
| JSON Schema | `https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/schemas/spec.schema.json` | Authoritative field-level schema (Draft 2020-12) |
| Field reference | `https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/SPEC.md` | Auto-generated tables of every field |
| Narrative guide | `https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/docs/writing-specs.md` | Worked examples, FAQ, edge cases (Indonesian prose, but structure is cross-language) |
| Full example | `https://github.com/Go-Routine-App/go-docs-generator/tree/main/examples/museum` | A complete spec for a real project |
| Changelog | `https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/CHANGELOG.md` | Breaking changes between versions |

---

## Self-check before declaring done

Before telling the user "spec is ready", confirm:

- [ ] Every endpoint in the spec corresponds to an actual route handler in the code.
- [ ] At least one `base_url` is set (either in `info.base_urls` or in every section).
- [ ] Validation passed: either `docs-gen validate` + `docs-gen lint` exit 0, OR `POST /docs/validate` returns `"ok": true`. Warnings OK to leave for later; errors must be fixed.
- [ ] If the project has distinct services with different hosts, each is a separate section with its own `base_url`.
- [ ] The spec, when served via `docs-gen -spec ...`, opens at `/docs` without a template error.
- [ ] The schema comment on line 1 (`# yaml-language-server: $schema=...`) is present for IDE support.
- [ ] No invented endpoints, no placeholder lorem ipsum, no leaked secrets.
