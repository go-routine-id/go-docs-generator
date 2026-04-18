package docs

//go:generate go run ../../cmd/gendocs

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
}

// OverviewCard represents a feature card on the overview page
type OverviewCard struct {
	Icon        string `yaml:"icon" json:"icon"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Content     string `yaml:"content,omitempty" json:"content,omitempty"`
}

// BaseURL represents a single environment base URL
type BaseURL struct {
	Label   string `yaml:"label" json:"label"`
	URL     string `yaml:"url" json:"url"`
	Default bool   `yaml:"default,omitempty" json:"default,omitempty"`
}

type InfoInfo struct {
	Title         string         `yaml:"title" json:"title"`
	Version       string         `yaml:"version" json:"version"`
	Description   string         `yaml:"description" json:"description"`
	BaseURL       string         `yaml:"base_url" json:"base_url"`
	BaseURLs      []BaseURL      `yaml:"base_urls" json:"base_urls"`
	OverviewCards []OverviewCard `yaml:"overview_cards" json:"overview_cards"`
}

// AuthMethod represents a single authentication method
type AuthMethod struct {
	Type          string   `yaml:"type" json:"type"`
	Header        string   `yaml:"header" json:"header"`
	Format        string   `yaml:"format" json:"format"`
	Source        string   `yaml:"source" json:"source"`
	Description   string   `yaml:"description" json:"description"`
	Note          string   `yaml:"note,omitempty" json:"note,omitempty"`
	TokenContains []string `yaml:"token_contains,omitempty" json:"token_contains,omitempty"`
}

type AuthenticationInfo struct {
	Type          string       `yaml:"type" json:"type"` // Deprecated: legacy single type
	Header        string       `yaml:"header" json:"header"` // Deprecated: legacy single header
	Source        string       `yaml:"source" json:"source"` // Deprecated: legacy single source
	TokenContains []string     `yaml:"token_contains" json:"token_contains"` // Deprecated: legacy
	Methods       []AuthMethod `yaml:"methods" json:"methods"`
}

// FlowMethodSteps groups steps for a specific auth method
// FlowOverviewStep represents a single step in the flow overview with expandable detail
type FlowOverviewStep struct {
	Title  string `yaml:"title" json:"title"`
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
}

type FlowMethodSteps struct {
	Type  string             `yaml:"type" json:"type"`
	Steps []FlowOverviewStep `yaml:"steps" json:"steps"`
}

type FlowOverviewInfo struct {
	Methods []FlowMethodSteps `yaml:"methods" json:"methods"`
	Note    string            `yaml:"note,omitempty" json:"note,omitempty"`
}

type SectionInfo struct {
	ID          string     `yaml:"id" json:"id"`
	Title       string     `yaml:"title" json:"title"`
	Description string     `yaml:"description" json:"description"`
	// BaseURL overrides Info.BaseURL for endpoints in this section.
	// Useful when a section describes a different service (e.g. account vs storage).
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	// BaseURLs overrides Info.BaseURLs for this section's API tester environment selector.
	BaseURLs  []BaseURL  `yaml:"base_urls,omitempty" json:"base_urls,omitempty"`
	Endpoints []Endpoint `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
}

// Guide represents a custom flow/guide (e.g. file upload flow)
type Guide struct {
	ID          string     `yaml:"id" json:"id"`
	Icon        string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Title       string     `yaml:"title" json:"title"`
	Description string     `yaml:"description" json:"description"`
	Flow        []FlowStep `yaml:"flow" json:"flow"`
}

// Screen represents a frontend/mobile screen and its API calls
type Screen struct {
	ID          string       `yaml:"id" json:"id"`
	Icon        string       `yaml:"icon,omitempty" json:"icon,omitempty"`
	Title       string       `yaml:"title" json:"title"`
	Description string       `yaml:"description" json:"description"`
	Image       string       `yaml:"image,omitempty" json:"image,omitempty"`
	Platform    []string     `yaml:"platform,omitempty" json:"platform,omitempty"`
	Calls       []ScreenCall `yaml:"calls" json:"calls"`
}

// ScreenCall represents a single API call made from a screen
type ScreenCall struct {
	Method  string `yaml:"method" json:"method"`
	Path    string `yaml:"path" json:"path"`
	Purpose string `yaml:"purpose" json:"purpose"`
	Trigger string `yaml:"trigger,omitempty" json:"trigger,omitempty"`
	Auth    string `yaml:"auth,omitempty" json:"auth,omitempty"`
	Notes   string `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type Endpoint struct {
	Name            string       `yaml:"name" json:"name"`
	Method          string       `yaml:"method" json:"method"`
	Path            string       `yaml:"path" json:"path"`
	Auth            string       `yaml:"auth" json:"auth"`
	Permission      string       `yaml:"permission,omitempty" json:"permission,omitempty"`
	Description     string       `yaml:"description" json:"description"`
	QueryParams     []QueryParam `yaml:"query_params,omitempty" json:"query_params,omitempty"`
	Body            []BodyField  `yaml:"body,omitempty" json:"body,omitempty"`
	ExampleBody     string       `yaml:"example_body,omitempty" json:"example_body,omitempty"`
	ExampleResponse string       `yaml:"example_response,omitempty" json:"example_response,omitempty"`
}

type QueryParam struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
	Description string `yaml:"description" json:"description"`
}

type BodyField struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Example     string `yaml:"example,omitempty" json:"example,omitempty"`
}

type FlowStep struct {
	Step             int           `yaml:"step" json:"step"`
	Title            string        `yaml:"title" json:"title"`
	Description      string        `yaml:"description,omitempty" json:"description,omitempty"`
	Endpoint         *FlowEndpoint `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Actions          []FlowAction  `yaml:"actions,omitempty" json:"actions,omitempty"`
	CurlExample      string        `yaml:"curl_example,omitempty" json:"curl_example,omitempty"`
	CurlExampleJWT   string        `yaml:"curl_example_jwt,omitempty" json:"curl_example_jwt,omitempty"`
	CurlExampleAPIKey string       `yaml:"curl_example_api_key,omitempty" json:"curl_example_api_key,omitempty"`
	ResponseExample  string        `yaml:"response_example,omitempty" json:"response_example,omitempty"`
}

type FlowEndpoint struct {
	Method      string      `yaml:"method" json:"method"`
	Path        string      `yaml:"path" json:"path"`
	Service     string      `yaml:"service" json:"service"`
	ContentType string      `yaml:"content_type,omitempty" json:"content_type,omitempty"`
	Auth        string      `yaml:"auth" json:"auth"`
	Permission  string      `yaml:"permission" json:"permission"`
	Fields      []BodyField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

type FlowAction struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Endpoint    string `yaml:"endpoint" json:"endpoint"`
}

type PermissionInfo struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

type FlowNodeInfo struct {
	ID       string   `yaml:"id" json:"id"`
	Label    string   `yaml:"label" json:"label"`
	Type     string   `yaml:"type" json:"type"`
	Color    string   `yaml:"color" json:"color"`
	Position Position `yaml:"position" json:"position"`
}

type Position struct {
	X float64 `yaml:"x" json:"x"`
	Y float64 `yaml:"y" json:"y"`
}

type FlowEdgeInfo struct {
	Source   string `yaml:"source" json:"source"`
	Target   string `yaml:"target" json:"target"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Animated bool   `yaml:"animated,omitempty" json:"animated,omitempty"`
	Color    string `yaml:"color" json:"color"`
	Style    string `yaml:"style,omitempty" json:"style,omitempty"`
}

// AuthMode represents an auth mode for the API tester
type AuthMode struct {
	Name        string `yaml:"name" json:"name"`
	Header      string `yaml:"header" json:"header"`
	Prefix      string `yaml:"prefix" json:"prefix"`
	Placeholder string `yaml:"placeholder" json:"placeholder"`
}

type APITesterDefaultsInfo struct {
	Methods   []string   `yaml:"methods" json:"methods"`
	AuthModes []AuthMode `yaml:"auth_modes" json:"auth_modes"`
}
