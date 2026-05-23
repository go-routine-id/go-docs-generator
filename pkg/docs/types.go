package docs

//go:generate go run ../../cmd/gendocs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

// Scalar holds a value written in YAML/JSON as a string, number, or boolean —
// e.g. a parameter `default`, where authors naturally write `default: 20`,
// `default: false`, or `default: "all"`. It is stored and rendered as its
// string form (the docs page only ever displays it as text). The custom
// JSONSchema makes raw validation accept all three primitive types instead of
// rejecting non-strings.
type Scalar string

// UnmarshalYAML accepts any scalar node and keeps its literal text.
func (s *Scalar) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a scalar (string, number, or boolean), got a %s", yamlKindName(node.Kind))
	}
	*s = Scalar(node.Value)
	return nil
}

// UnmarshalJSON accepts a JSON string, number, or boolean. Strings are
// unquoted; numbers and booleans keep their literal text.
func (s *Scalar) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = Scalar(str)
		return nil
	}
	*s = Scalar(string(data))
	return nil
}

// JSONSchema tells the schema generator (invopop/jsonschema) to accept any of
// the three primitive forms. anyOf (not oneOf) is deliberate: the integer 20
// matches both "number" and "integer", which would make oneOf fail.
func (Scalar) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "number"},
			{Type: "boolean"},
		},
	}
}

func yamlKindName(k yaml.Kind) string {
	switch k {
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.AliasNode:
		return "alias"
	default:
		return "non-scalar"
	}
}

// APISpec represents the complete API specification from YAML.
// Top-level fields are all optional at the file level — each overlay file in a
// multi-file spec directory typically only populates a subset. The merged
// document must ultimately have `info`, but any individual file may omit it.
type APISpec struct {
	Info              InfoInfo              `yaml:"info,omitempty" json:"info,omitempty" jsonschema_description:"Document metadata (title, version, base URLs, overview cards)."`
	Authentication    AuthenticationInfo    `yaml:"authentication,omitempty" json:"authentication,omitempty" jsonschema_description:"Authentication methods accepted by the API."`
	FlowOverview      FlowOverviewInfo      `yaml:"flow_overview,omitempty" json:"flow_overview,omitempty" jsonschema_description:"High-level auth/flow walkthrough shown on the overview page."`
	Sections          []SectionInfo         `yaml:"sections,omitempty" json:"sections,omitempty" jsonschema_description:"Endpoint groupings. Each section may override the document-level base URL."`
	Guides            []Guide               `yaml:"guides,omitempty" json:"guides,omitempty" jsonschema_description:"Step-by-step flows that span multiple endpoints (e.g. file upload)."`
	Screens           []Screen              `yaml:"screens,omitempty" json:"screens,omitempty" jsonschema_description:"Frontend/mobile screens and the API calls they make."`
	Permissions       []PermissionInfo      `yaml:"permissions,omitempty" json:"permissions,omitempty" jsonschema_description:"Permission names and descriptions referenced by endpoints."`
	Constraints       []string              `yaml:"constraints,omitempty" json:"constraints,omitempty" jsonschema_description:"Free-form rules or invariants of the API."`
	FlowDiagramNodes  []FlowNodeInfo        `yaml:"flow_diagram_nodes,omitempty" json:"flow_diagram_nodes,omitempty" jsonschema_description:"Nodes for the ReactFlow architecture diagram."`
	FlowDiagramEdges  []FlowEdgeInfo        `yaml:"flow_diagram_edges,omitempty" json:"flow_diagram_edges,omitempty" jsonschema_description:"Edges for the ReactFlow architecture diagram."`
	APITesterDefaults APITesterDefaultsInfo `yaml:"api_tester_defaults,omitempty" json:"api_tester_defaults,omitempty" jsonschema_description:"Defaults for the in-browser API tester (HTTP methods, auth modes)."`
	Events            []EventChannel        `yaml:"events,omitempty" json:"events,omitempty" jsonschema_description:"Async channels/topics the service publishes or consumes (Kafka, AMQP, MQTT, webhooks)."`
	Theme             Theme                 `yaml:"theme,omitempty" json:"theme,omitempty" jsonschema_description:"Branding overrides (title, logo, primary color). All fields optional."`
}

// Theme controls the branding of the rendered documentation page. All fields
// are optional; unset values fall back to Info.Title and built-in defaults.
type Theme struct {
	Title        string `yaml:"title,omitempty" json:"title,omitempty" jsonschema_description:"Overrides the title shown in the sidebar and mobile header."`
	LogoIcon     string `yaml:"logo_icon,omitempty" json:"logo_icon,omitempty" jsonschema_description:"Emoji or short string placed before the title."`
	LogoImage    string `yaml:"logo_image,omitempty" json:"logo_image,omitempty" jsonschema_description:"URL to a logo image shown in the sidebar header."`
	PrimaryColor string `yaml:"primary_color,omitempty" json:"primary_color,omitempty" jsonschema_description:"CSS color used for links, buttons, and highlights (overrides --primary)."`
	Favicon      string `yaml:"favicon,omitempty" json:"favicon,omitempty" jsonschema_description:"Browser favicon URL."`
}

// EventChannel documents an async messaging channel — a Kafka topic, AMQP
// queue, MQTT topic, webhook endpoint, or any pub/sub surface a service exposes.
type EventChannel struct {
	ID          string           `yaml:"id" json:"id" jsonschema_description:"Stable identifier used for anchor links."`
	Title       string           `yaml:"title" json:"title"`
	Description string           `yaml:"description,omitempty" json:"description,omitempty"`
	Protocol    string           `yaml:"protocol,omitempty" json:"protocol,omitempty" jsonschema_description:"Transport: kafka, amqp, mqtt, nats, webhook, sse, websocket, …"`
	Address     string           `yaml:"address,omitempty" json:"address,omitempty" jsonschema_description:"Protocol-specific address — topic name, queue name, URL."`
	Operations  []EventOperation `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// EventOperation is a single publish or subscribe action on an EventChannel.
type EventOperation struct {
	Type        string      `yaml:"type" json:"type" jsonschema_description:"publish or subscribe (from the documented service's perspective)."`
	Summary     string      `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Payload     []BodyField `yaml:"payload,omitempty" json:"payload,omitempty"`
	Example     string      `yaml:"example,omitempty" json:"example,omitempty"`
}

// OverviewCard represents a feature card on the overview page
type OverviewCard struct {
	Icon        string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Title       string `yaml:"title,omitempty" json:"title,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Content     string `yaml:"content,omitempty" json:"content,omitempty"`
}

// BaseURL represents a single environment base URL
type BaseURL struct {
	Label   string `yaml:"label,omitempty" json:"label,omitempty"`
	URL     string `yaml:"url,omitempty" json:"url,omitempty"`
	Default bool   `yaml:"default,omitempty" json:"default,omitempty"`
}

type InfoInfo struct {
	Title         string         `yaml:"title,omitempty" json:"title,omitempty"`
	Version       string         `yaml:"version,omitempty" json:"version,omitempty"`
	Description   string         `yaml:"description,omitempty" json:"description,omitempty"`
	BaseURL       string         `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	BaseURLs      []BaseURL      `yaml:"base_urls,omitempty" json:"base_urls,omitempty"`
	OverviewCards []OverviewCard `yaml:"overview_cards,omitempty" json:"overview_cards,omitempty"`
}

// AuthMethod represents a single authentication method
type AuthMethod struct {
	Type          string   `yaml:"type,omitempty" json:"type,omitempty"`
	Header        string   `yaml:"header,omitempty" json:"header,omitempty"`
	Format        string   `yaml:"format,omitempty" json:"format,omitempty"`
	Source        string   `yaml:"source,omitempty" json:"source,omitempty"`
	Description   string   `yaml:"description,omitempty" json:"description,omitempty"`
	Note          string   `yaml:"note,omitempty" json:"note,omitempty"`
	TokenContains []string `yaml:"token_contains,omitempty" json:"token_contains,omitempty"`
}

type AuthenticationInfo struct {
	Type          string       `yaml:"type,omitempty" json:"type,omitempty"`                     // Deprecated: legacy single type
	Header        string       `yaml:"header,omitempty" json:"header,omitempty"`                 // Deprecated: legacy single header
	Source        string       `yaml:"source,omitempty" json:"source,omitempty"`                 // Deprecated: legacy single source
	TokenContains []string     `yaml:"token_contains,omitempty" json:"token_contains,omitempty"` // Deprecated: legacy
	Methods       []AuthMethod `yaml:"methods,omitempty" json:"methods,omitempty"`
}

// FlowMethodSteps groups steps for a specific auth method
// FlowOverviewStep represents a single step in the flow overview with expandable detail
type FlowOverviewStep struct {
	Title  string `yaml:"title,omitempty" json:"title,omitempty"`
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
}

type FlowMethodSteps struct {
	Type  string             `yaml:"type,omitempty" json:"type,omitempty"`
	Steps []FlowOverviewStep `yaml:"steps,omitempty" json:"steps,omitempty"`
}

type FlowOverviewInfo struct {
	Methods []FlowMethodSteps `yaml:"methods,omitempty" json:"methods,omitempty"`
	Note    string            `yaml:"note,omitempty" json:"note,omitempty"`
}

type SectionInfo struct {
	ID          string `yaml:"id,omitempty" json:"id,omitempty"`
	Title       string `yaml:"title,omitempty" json:"title,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// BaseURL overrides Info.BaseURL for endpoints in this section.
	// Useful when a section describes a different service (e.g. account vs storage).
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	// BaseURLs overrides Info.BaseURLs for this section's API tester environment selector.
	BaseURLs  []BaseURL  `yaml:"base_urls,omitempty" json:"base_urls,omitempty"`
	Endpoints []Endpoint `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
}

// Guide represents a custom flow/guide (e.g. file upload flow)
type Guide struct {
	ID          string     `yaml:"id,omitempty" json:"id,omitempty"`
	Icon        string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Title       string     `yaml:"title,omitempty" json:"title,omitempty"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Flow        []FlowStep `yaml:"flow,omitempty" json:"flow,omitempty"`
}

// Screen represents a frontend/mobile screen and its API calls
type Screen struct {
	ID          string       `yaml:"id,omitempty" json:"id,omitempty"`
	Icon        string       `yaml:"icon,omitempty" json:"icon,omitempty"`
	Title       string       `yaml:"title,omitempty" json:"title,omitempty"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Image       string       `yaml:"image,omitempty" json:"image,omitempty"`
	Platform    []string     `yaml:"platform,omitempty" json:"platform,omitempty"`
	Calls       []ScreenCall `yaml:"calls,omitempty" json:"calls,omitempty"`
}

// ScreenCall represents a single API call made from a screen
type ScreenCall struct {
	Method  string `yaml:"method,omitempty" json:"method,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
	Purpose string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
	Trigger string `yaml:"trigger,omitempty" json:"trigger,omitempty"`
	Auth    string `yaml:"auth,omitempty" json:"auth,omitempty"`
	Notes   string `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type Endpoint struct {
	Name        string       `yaml:"name,omitempty" json:"name,omitempty"`
	Method      string       `yaml:"method,omitempty" json:"method,omitempty"`
	Path        string       `yaml:"path,omitempty" json:"path,omitempty"`
	Auth        string       `yaml:"auth,omitempty" json:"auth,omitempty"`
	Permission  string       `yaml:"permission,omitempty" json:"permission,omitempty"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Note        string       `yaml:"note,omitempty" json:"note,omitempty" jsonschema_description:"Caveat shown below the endpoint (e.g. 'unset fields are not updated')."`
	QueryParams []QueryParam `yaml:"query_params,omitempty" json:"query_params,omitempty"`
	Body        []BodyField  `yaml:"body,omitempty" json:"body,omitempty"`
	ExampleBody string       `yaml:"example_body,omitempty" json:"example_body,omitempty"`
	// Response is a short prose summary of what the endpoint returns. For a
	// concrete payload use ExampleResponse (or the authenticated/public split
	// when behaviour differs by auth state).
	Response                     string `yaml:"response,omitempty" json:"response,omitempty" jsonschema_description:"One-line prose description of the response shape."`
	ExampleResponse              string `yaml:"example_response,omitempty" json:"example_response,omitempty"`
	ExampleResponseAuthenticated string `yaml:"example_response_authenticated,omitempty" json:"example_response_authenticated,omitempty" jsonschema_description:"Example response when the caller is authenticated (for endpoints whose payload differs by auth state)."`
	ExampleResponsePublic        string `yaml:"example_response_public,omitempty" json:"example_response_public,omitempty" jsonschema_description:"Example response for an unauthenticated/public caller."`
}

type QueryParam struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     Scalar `yaml:"default,omitempty" json:"default,omitempty" jsonschema_description:"Default value. May be a string, number, or boolean."`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type BodyField struct {
	Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
	Type        string   `yaml:"type,omitempty" json:"type,omitempty"`
	Required    bool     `yaml:"required,omitempty" json:"required,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Example     string   `yaml:"example,omitempty" json:"example,omitempty"`
	Enum        []string `yaml:"enum,omitempty" json:"enum,omitempty" jsonschema_description:"Allowed values for this field."`
	MaxLength   int      `yaml:"max_length,omitempty" json:"max_length,omitempty" jsonschema_description:"Maximum string length, shown as a constraint."`
	Default     Scalar   `yaml:"default,omitempty" json:"default,omitempty" jsonschema_description:"Default value applied when the field is omitted. May be a string, number, or boolean."`
}

type FlowStep struct {
	Step              int           `yaml:"step,omitempty" json:"step,omitempty"`
	Title             string        `yaml:"title,omitempty" json:"title,omitempty"`
	Description       string        `yaml:"description,omitempty" json:"description,omitempty"`
	Endpoint          *FlowEndpoint `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Actions           []FlowAction  `yaml:"actions,omitempty" json:"actions,omitempty"`
	CurlExample       string        `yaml:"curl_example,omitempty" json:"curl_example,omitempty"`
	CurlExampleJWT    string        `yaml:"curl_example_jwt,omitempty" json:"curl_example_jwt,omitempty"`
	CurlExampleAPIKey string        `yaml:"curl_example_api_key,omitempty" json:"curl_example_api_key,omitempty"`
	ResponseExample   string        `yaml:"response_example,omitempty" json:"response_example,omitempty"`
	Tip               string        `yaml:"tip,omitempty" json:"tip,omitempty" jsonschema_description:"Highlighted hint shown at the end of the step."`
}

type FlowEndpoint struct {
	Method      string      `yaml:"method,omitempty" json:"method,omitempty"`
	Path        string      `yaml:"path,omitempty" json:"path,omitempty"`
	Service     string      `yaml:"service,omitempty" json:"service,omitempty"`
	ContentType string      `yaml:"content_type,omitempty" json:"content_type,omitempty"`
	Auth        string      `yaml:"auth,omitempty" json:"auth,omitempty"`
	AuthMethods []string    `yaml:"auth_methods,omitempty" json:"auth_methods,omitempty" jsonschema_description:"Accepted auth methods for this step (e.g. [Bearer JWT, X-API-Key])."`
	Permission  string      `yaml:"permission,omitempty" json:"permission,omitempty"`
	Fields      []BodyField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

type FlowAction struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Endpoint    string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Field       string `yaml:"field,omitempty" json:"field,omitempty" jsonschema_description:"The request field this action populates (e.g. image_media_id)."`
}

type PermissionInfo struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type FlowNodeInfo struct {
	ID       string   `yaml:"id,omitempty" json:"id,omitempty"`
	Label    string   `yaml:"label,omitempty" json:"label,omitempty"`
	Type     string   `yaml:"type,omitempty" json:"type,omitempty"`
	Color    string   `yaml:"color,omitempty" json:"color,omitempty"`
	Position Position `yaml:"position,omitempty" json:"position,omitempty"`
}

type Position struct {
	X float64 `yaml:"x,omitempty" json:"x,omitempty"`
	Y float64 `yaml:"y,omitempty" json:"y,omitempty"`
}

type FlowEdgeInfo struct {
	Source   string `yaml:"source,omitempty" json:"source,omitempty"`
	Target   string `yaml:"target,omitempty" json:"target,omitempty"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Animated bool   `yaml:"animated,omitempty" json:"animated,omitempty"`
	Color    string `yaml:"color,omitempty" json:"color,omitempty"`
	Style    string `yaml:"style,omitempty" json:"style,omitempty"`
}

// AuthMode represents an auth mode for the API tester
type AuthMode struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Header      string `yaml:"header,omitempty" json:"header,omitempty"`
	Prefix      string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Placeholder string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
}

type APITesterDefaultsInfo struct {
	Methods   []string   `yaml:"methods,omitempty" json:"methods,omitempty"`
	AuthModes []AuthMode `yaml:"auth_modes,omitempty" json:"auth_modes,omitempty"`
}
