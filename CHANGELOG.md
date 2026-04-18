# Changelog

## v2.0.0 — The "works for anyone" release

### Why

The generator was tightly coupled to Museum Digital Indonesia: a 2091-line
template string with Museum branding, a hand-written merge function that
required editing four files to add a new doc-type, and a single base URL per
project that could not model microservice boundaries. This release removes
those coupling points so teams outside Museum — monolith or microservice —
can adopt the tool without editing the source.

### Added

- **Per-section and per-endpoint `base_url` / `base_urls`** — one documentation page can now describe multiple services with different backends. Sections without their own base URL inherit from `info.base_urls` as before.
- **OpenAPI 3.x input** — point `-spec` at an existing `swagger.yaml` / `swagger.json` and it is auto-detected and projected onto the internal model. No conversion step required.
- **OpenAPI 3.0 output** — new `/docs/openapi` endpoint exports the internal spec as OpenAPI for Postman, Insomnia, Redocly, etc.
- **Events / async type** — `events[]` top-level field documents Kafka topics, AMQP queues, MQTT topics, webhooks, and other async surfaces. Rendered as a dedicated sidebar section.
- **`theme` block** — override title, logo, primary color, and favicon without touching the template.
- **Subcommands** — `docs-gen validate <path>` (CI-friendly) and `docs-gen init [dir]` (scaffold).
- **JSON Schema** — `schemas/spec.schema.json` (Draft 2020-12) enables IDE autocomplete via `# yaml-language-server: $schema=...`.
- **SPEC.md** — auto-generated reference of every field, regenerated with `make generate`.
- **Auto-layout flow diagram** — when all nodes have `position: {x:0, y:0}`, dagre computes a tree layout client-side. Manual positioning still wins per-node.
- **Structured logging** via `log/slog` — text in dev, JSON under `GIN_MODE=release` or `LOG_FORMAT=json`.
- **Golden render test + structural invariants** — regressions in rendering fail CI.
- **Loader unit tests** — merge semantics, auto-include, multi-project discovery.
- **CI + Release workflows** — tests on every push, binaries and ghcr image on tag.

### Changed

- **`mergeSpec` is now reflection-driven.** Slice fields are appended, nested structs are recursed, scalar fields are overridden when the overlay value is non-zero. Adding a new top-level field no longer requires editing the merger.
- **Template split** — `pkg/docs/template.go` (2091 lines) is now `pkg/docs/templates/*.gohtml` (4 files) loaded via `embed.FS`. Editable with syntax highlighting.
- **Museum spec moved** from `spec/` to `examples/museum/`. `spec/` now contains a generic starter template. The Museum fixture is preserved under `pkg/docs/testdata/specs/museum/` for golden tests.
- **README rewritten** for a general audience. No more Museum-specific paths or domain names.
- **Docker image** is now distroless-based and built against `./spec`. Tagged `docs-generator` (was `museum-docs`).
- **Banner and logo** are neutral by default — `📖` instead of `🏛️`. Use `theme.logo_icon` / `theme.logo_image` to brand.

### Removed

- `museum-docs.service` (systemd unit) and `install-systemd` Makefile target. Deployment is now up to the operator (see README for PM2 or Docker examples).
- Buggy duplicate-append logic in the old `mergeSpec` for `Info.OverviewCards`.

### Breaking changes

1. **Merge semantics**: an overlay file whose `info.title` was set previously replaced the whole `info` block. Now `info` is merged per-field; scalars win only when the overlay is non-zero. Specs that never used this quirk (the common case) are unaffected.
2. **Spec path default remains `./spec/index.yaml`**, but `spec/` now ships a generic example. Set `-spec ./examples/museum/index.yaml` to run the old Museum docs.
3. **Docker image name** changed from `museum-docs` to `docs-generator`.

### Migration

- If you kept a Museum-style spec in `spec/`, move it to `examples/<yourproject>/` and pass the path explicitly via `-spec`.
- If your overlay files relied on "set `info.title` to replace the whole info object", switch to setting each field you actually want to override; zero values no longer clobber the base.
- If your deployment used the systemd unit, see the README's PM2 / Docker sections for replacements.
