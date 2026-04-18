# Spec Reference

> Auto-generated from Go structs in `pkg/docs`. **Do not edit by hand.**
>
> Regenerate with: `go run ./cmd/gendocs`

This document describes the shape of a Docs Generator spec file (`spec/index.yaml` or equivalent).
The same schema is also published as JSON Schema Draft 2020-12 in [`schemas/spec.schema.json`](schemas/spec.schema.json) and can be referenced from YAML files with:

```yaml
# yaml-language-server: $schema=./schemas/spec.schema.json
```

> 💡 For a narrative guide — when to use each mode, worked monolith-vs-microservice examples, conventions, and FAQ — see **[`docs/writing-specs.md`](docs/writing-specs.md)**.

## Merge rules (multi-file specs)

When a spec directory contains multiple YAML files, they are merged into a single document:

- **Slice fields** (e.g. `sections`, `guides`, `screens`): appended — every file contributes.
- **Nested object fields** (e.g. `info`): merged per-field — overlay non-zero values override.
- **Scalar fields** (strings, numbers, booleans): overlay value wins when non-zero.

## Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `info` | [`InfoInfo`](#infoinfo) | no | Document metadata (title, version, base URLs, overview cards). |
| `authentication` | [`AuthenticationInfo`](#authenticationinfo) | no | Authentication methods accepted by the API. |
| `flow_overview` | [`FlowOverviewInfo`](#flowoverviewinfo) | no | High-level auth/flow walkthrough shown on the overview page. |
| `sections` | array<[`SectionInfo`](#sectioninfo)> | no | Endpoint groupings. Each section may override the document-level base URL. |
| `guides` | array<[`Guide`](#guide)> | no | Step-by-step flows that span multiple endpoints (e.g. file upload). |
| `screens` | array<[`Screen`](#screen)> | no | Frontend/mobile screens and the API calls they make. |
| `permissions` | array<[`PermissionInfo`](#permissioninfo)> | no | Permission names and descriptions referenced by endpoints. |
| `constraints` | array<`string`> | no | Free-form rules or invariants of the API. |
| `flow_diagram_nodes` | array<[`FlowNodeInfo`](#flownodeinfo)> | no | Nodes for the ReactFlow architecture diagram. |
| `flow_diagram_edges` | array<[`FlowEdgeInfo`](#flowedgeinfo)> | no | Edges for the ReactFlow architecture diagram. |
| `api_tester_defaults` | [`APITesterDefaultsInfo`](#apitesterdefaultsinfo) | no | Defaults for the in-browser API tester (HTTP methods, auth modes). |
| `events` | array<[`EventChannel`](#eventchannel)> | no | Async channels/topics the service publishes or consumes (Kafka, AMQP, MQTT, webhooks). |
| `theme` | [`Theme`](#theme) | no | Branding overrides (title, logo, primary color). All fields optional. |

## Nested types

### `APITesterDefaultsInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `methods` | array<`string`> | no | — |
| `auth_modes` | array<[`AuthMode`](#authmode)> | no | — |

### `AuthMethod`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | no | — |
| `header` | `string` | no | — |
| `format` | `string` | no | — |
| `source` | `string` | no | — |
| `description` | `string` | no | — |
| `note` | `string` | no | — |
| `token_contains` | array<`string`> | no | — |

### `AuthMode`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | no | — |
| `header` | `string` | no | — |
| `prefix` | `string` | no | — |
| `placeholder` | `string` | no | — |

### `AuthenticationInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | no | — |
| `header` | `string` | no | — |
| `source` | `string` | no | — |
| `token_contains` | array<`string`> | no | — |
| `methods` | array<[`AuthMethod`](#authmethod)> | no | — |

### `BaseURL`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `label` | `string` | no | — |
| `url` | `string` | no | — |
| `default` | `boolean` | no | — |

### `BodyField`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | no | — |
| `type` | `string` | no | — |
| `required` | `boolean` | no | — |
| `description` | `string` | no | — |
| `example` | `string` | no | — |

### `Endpoint`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | no | — |
| `method` | `string` | no | — |
| `path` | `string` | no | — |
| `auth` | `string` | no | — |
| `permission` | `string` | no | — |
| `description` | `string` | no | — |
| `query_params` | array<[`QueryParam`](#queryparam)> | no | — |
| `body` | array<[`BodyField`](#bodyfield)> | no | — |
| `example_body` | `string` | no | — |
| `example_response` | `string` | no | — |

### `EventChannel`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | **yes** | Stable identifier used for anchor links. |
| `title` | `string` | **yes** | — |
| `description` | `string` | no | — |
| `protocol` | `string` | no | Transport: kafka, amqp, mqtt, nats, webhook, sse, websocket, … |
| `address` | `string` | no | Protocol-specific address — topic name, queue name, URL. |
| `operations` | array<[`EventOperation`](#eventoperation)> | no | — |

### `EventOperation`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | **yes** | publish or subscribe (from the documented service's perspective). |
| `summary` | `string` | no | — |
| `description` | `string` | no | — |
| `payload` | array<[`BodyField`](#bodyfield)> | no | — |
| `example` | `string` | no | — |

### `FlowAction`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | no | — |
| `description` | `string` | no | — |
| `endpoint` | `string` | no | — |

### `FlowEdgeInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | `string` | no | — |
| `target` | `string` | no | — |
| `label` | `string` | no | — |
| `animated` | `boolean` | no | — |
| `color` | `string` | no | — |
| `style` | `string` | no | — |

### `FlowEndpoint`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | `string` | no | — |
| `path` | `string` | no | — |
| `service` | `string` | no | — |
| `content_type` | `string` | no | — |
| `auth` | `string` | no | — |
| `permission` | `string` | no | — |
| `fields` | array<[`BodyField`](#bodyfield)> | no | — |

### `FlowMethodSteps`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | no | — |
| `steps` | array<[`FlowOverviewStep`](#flowoverviewstep)> | no | — |

### `FlowNodeInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | no | — |
| `label` | `string` | no | — |
| `type` | `string` | no | — |
| `color` | `string` | no | — |
| `position` | [`Position`](#position) | no | — |

### `FlowOverviewInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `methods` | array<[`FlowMethodSteps`](#flowmethodsteps)> | no | — |
| `note` | `string` | no | — |

### `FlowOverviewStep`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | no | — |
| `detail` | `string` | no | — |

### `FlowStep`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `step` | `integer` | no | — |
| `title` | `string` | no | — |
| `description` | `string` | no | — |
| `endpoint` | [`FlowEndpoint`](#flowendpoint) | no | — |
| `actions` | array<[`FlowAction`](#flowaction)> | no | — |
| `curl_example` | `string` | no | — |
| `curl_example_jwt` | `string` | no | — |
| `curl_example_api_key` | `string` | no | — |
| `response_example` | `string` | no | — |

### `Guide`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | no | — |
| `icon` | `string` | no | — |
| `title` | `string` | no | — |
| `description` | `string` | no | — |
| `flow` | array<[`FlowStep`](#flowstep)> | no | — |

### `InfoInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | no | — |
| `version` | `string` | no | — |
| `description` | `string` | no | — |
| `base_url` | `string` | no | — |
| `base_urls` | array<[`BaseURL`](#baseurl)> | no | — |
| `overview_cards` | array<[`OverviewCard`](#overviewcard)> | no | — |

### `OverviewCard`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `icon` | `string` | no | — |
| `title` | `string` | no | — |
| `description` | `string` | no | — |
| `content` | `string` | no | — |

### `PermissionInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | no | — |
| `description` | `string` | no | — |

### `Position`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `x` | `number` | no | — |
| `y` | `number` | no | — |

### `QueryParam`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | no | — |
| `type` | `string` | no | — |
| `required` | `boolean` | no | — |
| `default` | `string` | no | — |
| `description` | `string` | no | — |

### `Screen`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | no | — |
| `icon` | `string` | no | — |
| `title` | `string` | no | — |
| `description` | `string` | no | — |
| `image` | `string` | no | — |
| `platform` | array<`string`> | no | — |
| `calls` | array<[`ScreenCall`](#screencall)> | no | — |

### `ScreenCall`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | `string` | no | — |
| `path` | `string` | no | — |
| `purpose` | `string` | no | — |
| `trigger` | `string` | no | — |
| `auth` | `string` | no | — |
| `notes` | `string` | no | — |

### `SectionInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | no | — |
| `title` | `string` | no | — |
| `description` | `string` | no | — |
| `base_url` | `string` | no | — |
| `base_urls` | array<[`BaseURL`](#baseurl)> | no | — |
| `endpoints` | array<[`Endpoint`](#endpoint)> | no | — |

### `Theme`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | no | Overrides the title shown in the sidebar and mobile header. |
| `logo_icon` | `string` | no | Emoji or short string placed before the title. |
| `logo_image` | `string` | no | URL to a logo image shown in the sidebar header. |
| `primary_color` | `string` | no | CSS color used for links, buttons, and highlights (overrides --primary). |
| `favicon` | `string` | no | Browser favicon URL. |

