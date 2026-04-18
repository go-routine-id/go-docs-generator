# Spec Reference

> Auto-generated from Go structs in `pkg/docs`. **Do not edit by hand.**
>
> Regenerate with: `go run ./cmd/gendocs`

This document describes the shape of a Docs Generator spec file (`spec/index.yaml` or equivalent).
The same schema is also published as JSON Schema Draft 2020-12 in [`schemas/spec.schema.json`](schemas/spec.schema.json) and can be referenced from YAML files with:

```yaml
# yaml-language-server: $schema=./schemas/spec.schema.json
```

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

## Nested types

### `APITesterDefaultsInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `methods` | array<`string`> | **yes** | — |
| `auth_modes` | array<[`AuthMode`](#authmode)> | **yes** | — |

### `AuthMethod`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | **yes** | — |
| `header` | `string` | **yes** | — |
| `format` | `string` | **yes** | — |
| `source` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `note` | `string` | no | — |
| `token_contains` | array<`string`> | no | — |

### `AuthMode`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | **yes** | — |
| `header` | `string` | **yes** | — |
| `prefix` | `string` | **yes** | — |
| `placeholder` | `string` | **yes** | — |

### `AuthenticationInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | **yes** | — |
| `header` | `string` | **yes** | — |
| `source` | `string` | **yes** | — |
| `token_contains` | array<`string`> | **yes** | — |
| `methods` | array<[`AuthMethod`](#authmethod)> | **yes** | — |

### `BaseURL`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `label` | `string` | **yes** | — |
| `url` | `string` | **yes** | — |
| `default` | `boolean` | no | — |

### `BodyField`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | **yes** | — |
| `type` | `string` | **yes** | — |
| `required` | `boolean` | **yes** | — |
| `description` | `string` | no | — |
| `example` | `string` | no | — |

### `Endpoint`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | **yes** | — |
| `method` | `string` | **yes** | — |
| `path` | `string` | **yes** | — |
| `auth` | `string` | **yes** | — |
| `permission` | `string` | no | — |
| `description` | `string` | **yes** | — |
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
| `type` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `endpoint` | `string` | **yes** | — |

### `FlowEdgeInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | `string` | **yes** | — |
| `target` | `string` | **yes** | — |
| `label` | `string` | no | — |
| `animated` | `boolean` | no | — |
| `color` | `string` | **yes** | — |
| `style` | `string` | no | — |

### `FlowEndpoint`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | `string` | **yes** | — |
| `path` | `string` | **yes** | — |
| `service` | `string` | **yes** | — |
| `content_type` | `string` | no | — |
| `auth` | `string` | **yes** | — |
| `permission` | `string` | **yes** | — |
| `fields` | array<[`BodyField`](#bodyfield)> | no | — |

### `FlowMethodSteps`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | **yes** | — |
| `steps` | array<[`FlowOverviewStep`](#flowoverviewstep)> | **yes** | — |

### `FlowNodeInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | **yes** | — |
| `label` | `string` | **yes** | — |
| `type` | `string` | **yes** | — |
| `color` | `string` | **yes** | — |
| `position` | [`Position`](#position) | **yes** | — |

### `FlowOverviewInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `methods` | array<[`FlowMethodSteps`](#flowmethodsteps)> | **yes** | — |
| `note` | `string` | no | — |

### `FlowOverviewStep`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | **yes** | — |
| `detail` | `string` | no | — |

### `FlowStep`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `step` | `integer` | **yes** | — |
| `title` | `string` | **yes** | — |
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
| `id` | `string` | **yes** | — |
| `icon` | `string` | no | — |
| `title` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `flow` | array<[`FlowStep`](#flowstep)> | **yes** | — |

### `InfoInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | **yes** | — |
| `version` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `base_url` | `string` | **yes** | — |
| `base_urls` | array<[`BaseURL`](#baseurl)> | **yes** | — |
| `overview_cards` | array<[`OverviewCard`](#overviewcard)> | **yes** | — |

### `OverviewCard`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `icon` | `string` | **yes** | — |
| `title` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `content` | `string` | no | — |

### `PermissionInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |

### `Position`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `x` | `number` | **yes** | — |
| `y` | `number` | **yes** | — |

### `QueryParam`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | **yes** | — |
| `type` | `string` | **yes** | — |
| `required` | `boolean` | **yes** | — |
| `default` | `string` | no | — |
| `description` | `string` | **yes** | — |

### `Screen`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | **yes** | — |
| `icon` | `string` | no | — |
| `title` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `image` | `string` | no | — |
| `platform` | array<`string`> | no | — |
| `calls` | array<[`ScreenCall`](#screencall)> | **yes** | — |

### `ScreenCall`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | `string` | **yes** | — |
| `path` | `string` | **yes** | — |
| `purpose` | `string` | **yes** | — |
| `trigger` | `string` | no | — |
| `auth` | `string` | no | — |
| `notes` | `string` | no | — |

### `SectionInfo`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | **yes** | — |
| `title` | `string` | **yes** | — |
| `description` | `string` | **yes** | — |
| `base_url` | `string` | no | — |
| `base_urls` | array<[`BaseURL`](#baseurl)> | no | — |
| `endpoints` | array<[`Endpoint`](#endpoint)> | no | — |

