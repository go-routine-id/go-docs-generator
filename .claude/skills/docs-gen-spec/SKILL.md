---
name: docs-gen-spec
description: Use when authoring, editing, validating, generating, or troubleshooting any spec consumed by go-docs-generator — typically `api-spec.yaml`, `spec/index.yaml`, files under `spec/`, `web/docs/`, `examples/<project>/`, or any YAML the user calls a "docs spec" / "API spec" for the docs page. Triggers on mentions of `docs-generator`, `docs-gen`, `/docs page` rendering, `api_tester_defaults`, `auth_modes`, "tester crashes", `loadCredentials`, `JSON.parse("null")`, `flow_overview`, `flow_diagram_*`, `base_urls`, or schema/lint failures from this tool.
---

# Authoring specs for go-docs-generator

The user is editing a spec consumed by `docs-generator`. The schema is permissive — many mistakes pass `validate` and crash silently in the browser. Follow the steps below; do not freelance from memory.

## 1. Load the canonical guide before writing anything

Run **once** per task and keep the output in context:

```bash
docs-gen prompt
```

That prints the full, current `AGENTS.md`. If `docs-gen` is not on PATH, `cat AGENTS.md` from the docs-generator repo, or fetch `<docs-host>/docs/agents`.

Do not author from memory of an older version of the schema — fields have been renamed, removed, and tightened.

## 2. The non-obvious rules that bite every time

These are the failure modes we have actually shipped bugs for. Reading AGENTS.md alone misses them:

1. **`auth_modes` is mandatory if ANY endpoint has `auth` other than `none` / empty.** Skipping it makes the rendered page crash on load with `Cannot read properties of null (reading 'forEach')`. The lint check now errors on this.

2. **`auth: required` and `auth: optional` are UI badge keywords** — they render a badge but do **not** auto-bind to an auth mode. Either:
   - Use a real auth-mode name (e.g. `auth: "JWT Bearer"`) that exactly matches `api_tester_defaults.auth_modes[].name`, or
   - Use `auth: required` / `auth: optional` for the badge AND list a matching entry in `auth_modes` so the tester can attach credentials.

3. **Unknown fields are dropped silently.** Common mistakes that "look fine":
   - `api_tester_defaults.default_url`, `api_tester_defaults.quick_tests` — not in the schema, ignored.
   - `authentication.type` / `header` / `source` flat — legacy format, still parsed for backward compat but deprecated. Use `authentication.methods: [...]`.
   - `flow_overview.steps:` bare array — legacy. Use `flow_overview.methods[].steps`.

4. **`base_url` (singular) vs `base_urls` (plural).** Plural enables the environment selector (Production/Staging/Local). Singular is a one-shot fallback. Prefer plural for any non-trivial API.

5. **`prefix` on every `auth_modes[]` entry must be set explicitly** — even to `""`. Omitting it renders `undefined<token>` in the Authorization header.

## 3. Validate before declaring done

Both checks. Validate-only is not enough; lint catches the runtime traps.

```bash
docs-gen validate <path-to-spec>
docs-gen lint <path-to-spec>
```

Exit code must be `0` for both. If lint exits `1`, fix and rerun — do not ship a spec that lint flags as error.

If `docs-gen` is not on PATH, run from the docs-generator checkout:

```bash
go run ./cmd/server validate <path>
go run ./cmd/server lint <path>
```

## 4. Visual smoke test for non-trivial changes

If the user's edit touches `api_tester_defaults`, `authentication`, `flow_overview`, `flow_diagram_*`, or any rendered surface, also boot the server and load `/docs` in a browser (or `curl /docs | grep`) to confirm:

- No `JSON.parse("null")` without `|| []` immediately after.
- The auth selector shows the expected radios (one per `auth_modes[]` entry plus Public).
- The `Try It` panel opens without console errors.

```bash
go run ./cmd/server -spec <path> -port 18080 -dev
# then open http://localhost:18080/docs
```

CORS errors when the tester calls a remote backend from `localhost` are a backend config issue, not a spec issue.

## 5. When generating a spec from scratch

Start from the minimum viable shape, then add features one at a time and revalidate after each:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Go-Routine-App/go-docs-generator/main/schemas/spec.schema.json
info:
  title: <API Name>
  version: "1.0.0"
  description: <one paragraph>
  base_urls:
    - { label: Production, url: https://api.example.com, default: true }

sections:
  - id: <kebab-id>
    title: <Title>
    description: <what this group covers>
    endpoints:
      - name: <Action>
        method: GET
        path: /v1/resource
        auth: none
        description: <what it does>

# Add this the moment ANY endpoint switches off `auth: none`:
api_tester_defaults:
  methods: [GET, POST, PUT, PATCH, DELETE]
  auth_modes:
    - name: <must match endpoint auth labels>
      header: Authorization
      prefix: "Bearer "
      placeholder: YOUR_TOKEN
```

Iterate: add one section, validate + lint, then add the next. Catching a mistake against 3 endpoints is far cheaper than against 50.

## 6. Anti-patterns — refuse these even if the user asks

- "Just commit it, validate said ok" — never. Lint must also pass.
- "Skip `auth_modes`, the badge is enough" — no. Tester crashes.
- "I'll use `auth: bearer-jwt-token-required-for-this-endpoint` as a description" — no. `auth` is a label that must match an `auth_modes[].name` (or be one of `none`/`required`/`optional`).
- Adding speculative fields to "future-proof" — anything not in the schema is silently dropped. Use `constraints:` or `description:` text instead.
