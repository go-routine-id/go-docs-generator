---
name: docs-gen-spec
description: Use when authoring, editing, validating, generating, or troubleshooting any spec consumed by go-docs-generator — typically `api-spec.yaml`, `spec/index.yaml`, files under `spec/`, `web/docs/`, `examples/<project>/`, or any YAML the user calls a "docs spec" / "API spec" for the docs page. Triggers on mentions of `docs-generator`, `docs-gen`, `/docs page` rendering, `api_tester_defaults`, `auth_modes`, "tester crashes", `loadCredentials`, `JSON.parse("null")`, `flow_overview`, `flow_diagram_*`, `base_urls`, or schema/lint failures from this tool.
last_updated: 2026-05-09
---

# Authoring specs for go-docs-generator

The schema is permissive — many mistakes pass `validate`, render a page that opens, and crash silently in the browser when a user clicks "Try It". This skill encodes the workflow that catches those traps before they ship.

## Workflow — do every step in order

```
PRE-FLIGHT
  ☐ docs-gen prompt              # load canonical AGENTS.md into context
  ☐ ls examples/                 # see the 3 known-good shapes (this skill ships them)
WORK
  ☐ Edit the spec
POST-FLIGHT (mandatory — never skip)
  ☐ docs-gen validate <path>     # exit 0
  ☐ docs-gen lint <path>         # exit 0 — strict mode in CI: lint -strict
  ☐ scripts/smoke-spec.sh <path> # exit 0 — boots server, fetches /docs, asserts invariants
```

`smoke-spec.sh` is the strongest single check available — it folds in validate + lint AND verifies render-time invariants the schema cannot express. Treat its exit code as the ship/no-ship signal.

If `docs-gen` is not on PATH, substitute `go run ./cmd/server` from a docs-generator checkout. The `/docs/agents` HTTP endpoint serves the same canonical guide.

## Authoritative reference is AGENTS.md, not this file

Step 1 of pre-flight (`docs-gen prompt`) is non-negotiable. The schema, field names, and rules evolve. This SKILL.md is updated by hand and dated in the frontmatter (`last_updated`). If anything below contradicts the just-loaded `AGENTS.md`, **AGENTS.md wins**. Flag the divergence to the user so this file can be refreshed.

## The five gotchas that bypass `validate`

1. **`auth_modes` is mandatory once any endpoint declares `auth` other than `none` / empty.** Skipping it makes the rendered page crash on load with `Cannot read properties of null (reading 'forEach')`. The lint check now errors on this — the smoke script catches it.

2. **`auth: required` and `auth: optional` are UI-badge keywords**, not auth-mode references. They render a badge but do not auto-bind to credentials. Either:
   - Use the real auth-mode name (e.g. `auth: "JWT Bearer"`) that exactly matches `api_tester_defaults.auth_modes[].name` (case-sensitive), or
   - Use `auth: required` / `auth: optional` for the badge AND list a matching entry in `auth_modes` so the tester can attach credentials.

3. **Unknown fields are dropped silently** — there is no "did you mean" hint. Common mistakes:
   - `api_tester_defaults.default_url`, `api_tester_defaults.quick_tests` — not in the schema, ignored.
   - `authentication.type` / `header` / `source` flat — legacy format, parsed for backward compat but deprecated. Use `authentication.methods: [...]`.
   - `flow_overview.steps:` bare array — legacy. Use `flow_overview.methods[].steps`.

4. **`base_url` (singular) vs `base_urls` (plural).** Plural enables the environment selector (Production / Staging / Local). Singular is a one-shot fallback. Prefer plural for any non-trivial API.

5. **Every `auth_modes[]` entry MUST set `prefix` explicitly** — even to `""`. Omitting it renders `Authorization: undefined<token>` in the tester. The smoke script catches this.

## Symptom → diagnosis table

When the user reports a problem, map it before opening the spec:

| Symptom | Cause | Fix |
|---|---|---|
| Console: `Cannot read properties of null (reading 'forEach')` at `loadCredentials` | `auth_modes` empty / missing | Add `api_tester_defaults.auth_modes[]` matching every endpoint's `auth` |
| Tester shows only one giant "Public" radio | `auth_modes` empty (page didn't crash thanks to the post-9b0a845 guard) | Same as above |
| `JSON.parse("null")` in HTML source without `\|\| []` after | Running docs-generator older than commit `9b0a845` | Update binary |
| Tester "Send" returns `Error: Failed to fetch` | Browser CORS — backend doesn't allow the dev origin | **Out of scope** — fix the backend's CORS allow-list |
| Endpoint missing from sidebar | YAML indent / dash error in `endpoints:` | Re-validate; fix YAML structure |
| `Authorization: undefined<token>` sent | `prefix` field omitted from the matching `auth_modes[]` entry | Add `prefix: "Bearer "` (or `prefix: ""` for bare keys) |
| Field appears to "do nothing" | Field name not in schema → silently dropped | Cross-check against `docs-gen prompt` cheat sheet |
| `validate` says ok but page misbehaves | `validate` is JSON Schema only — does not catch the bugs above | Always run `lint` and `smoke-spec.sh` in addition |

## Generating from scratch

Do not synthesize from memory. Copy the closest example shipped with this skill, then edit:

| If the API is… | Start from |
|---|---|
| Public-only / read-only / no auth | `examples/01-public-only.yaml` |
| Single auth method (JWT or API key alone) | `examples/02-jwt-only.yaml` |
| Multi-auth (JWT for users + API key for service-to-service) | `examples/03-jwt-plus-apikey.yaml` |

These three are part of the skill's automated test suite — they pass `validate`, `lint`, and `smoke-spec.sh` on every CI run, so they are guaranteed correct against the current schema. Iterate one section at a time and re-run smoke after each addition; catching a mistake against 3 endpoints is far cheaper than against 50.

## Two contexts you might be in

The right thing to do depends on which repo is checked out:

**A. Working inside the docs-generator repo itself** (editing `pkg/docs/`, templates, lint rules, examples, schema).
- Run `go test ./...` after every code change — golden tests will fail loudly if a template change drifts.
- If templates change: regenerate the golden with `go test ./pkg/docs -run TestRender -update-golden` and review the diff in git.
- If types change: run `make generate` to refresh `schemas/spec.schema.json` and `SPEC.md`, plus `cp AGENTS.md cmd/server/agents.md` so the embedded copy stays in sync (handled by `make generate`).
- Smoke the example specs in `.claude/skills/docs-gen-spec/examples/` — those guard the contract.

**B. Working inside a consumer repo** (e.g. editing `web/docs/api-spec.yaml` for some service).
- No Go knowledge needed. Just edit + validate + lint + smoke.
- Use the symlinked `~/.claude/skills/docs-gen-spec/scripts/smoke-spec.sh` — it auto-finds the docs-generator repo via the symlink chain and uses `go run ./cmd/server` if the binary isn't on PATH.
- Do NOT modify `pkg/docs/` from here — that's a different repo. Open a PR in docs-generator instead.

## Anti-patterns — refuse these even if the user asks

- "Just commit it, validate said ok" — never. Lint and smoke must also pass. `validate` alone is a known-incomplete signal.
- "Skip `auth_modes`, the badge alone is enough" — no. The tester crashes (or, post-fix, becomes useless because Public is the only available radio).
- "I'll write `auth: bearer-jwt-token-required-for-this-endpoint` as a description" — no. `auth` is a label that must match an `auth_modes[].name` (or be one of the special keywords `none` / `required` / `optional`).
- "Add a `quick_tests:` block under `api_tester_defaults` to pre-populate buttons" — no. That field does not exist in the schema and is silently dropped. If you need fixed example calls, put them in `endpoints[].example_body` and `example_response`.
- "Use this field, I'll add it to the schema later" — no. Either the field exists today (check `docs-gen prompt`) or it doesn't ship until the schema does.

## Self-check before declaring done

```
☐ docs-gen prompt                    output reviewed (or already in context)
☐ docs-gen validate <path>           exit 0
☐ docs-gen lint <path>               exit 0 (no errors; warnings reviewed)
☐ scripts/smoke-spec.sh <path>       exit 0 (all invariants green)
☐ Visual sanity check                /docs renders the new section, tester opens, radios check
☐ Diff reviewed                      no accidental schema-version downgrade, no removed fields
```

If any box is unchecked, the spec is not done.
