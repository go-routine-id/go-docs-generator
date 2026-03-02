package docs

// APISpec represents the complete API specification from YAML
type APISpec struct {
	Info              InfoInfo              `yaml:"info" json:"info"`
	Authentication    AuthenticationInfo    `yaml:"authentication" json:"authentication"`
	FlowOverview      FlowOverviewInfo      `yaml:"flow_overview" json:"flow_overview"`
	Sections          []SectionInfo         `yaml:"sections" json:"sections"`
	Permissions       []PermissionInfo      `yaml:"permissions" json:"permissions"`
	Constraints       []string              `yaml:"constraints" json:"constraints"`
	FlowDiagramNodes  []FlowNodeInfo        `yaml:"flow_diagram_nodes" json:"flow_diagram_nodes"`
	FlowDiagramEdges  []FlowEdgeInfo        `yaml:"flow_diagram_edges" json:"flow_diagram_edges"`
	APITesterDefaults APITesterDefaultsInfo `yaml:"api_tester_defaults" json:"api_tester_defaults"`
}

type InfoInfo struct {
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
	BaseURL     string `yaml:"base_url" json:"base_url"`
}

type AuthenticationInfo struct {
	Type          string   `yaml:"type" json:"type"`
	Header        string   `yaml:"header" json:"header"`
	Source        string   `yaml:"source" json:"source"`
	TokenContains []string `yaml:"token_contains" json:"token_contains"`
}

type FlowOverviewInfo struct {
	Steps []string `yaml:"steps" json:"steps"`
}

type SectionInfo struct {
	ID          string     `yaml:"id" json:"id"`
	Title       string     `yaml:"title" json:"title"`
	Description string     `yaml:"description" json:"description"`
	Endpoints   []Endpoint `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
	Flow        []FlowStep `yaml:"flow,omitempty" json:"flow,omitempty"`
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
	Step            int           `yaml:"step" json:"step"`
	Title           string        `yaml:"title" json:"title"`
	Description     string        `yaml:"description,omitempty" json:"description,omitempty"`
	Endpoint        *FlowEndpoint `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Actions         []FlowAction  `yaml:"actions,omitempty" json:"actions,omitempty"`
	CurlExample     string        `yaml:"curl_example,omitempty" json:"curl_example,omitempty"`
	ResponseExample string        `yaml:"response_example,omitempty" json:"response_example,omitempty"`
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

type APITesterDefaultsInfo struct {
	DefaultURL string      `yaml:"default_url" json:"default_url"`
	QuickTests []QuickTest `yaml:"quick_tests" json:"quick_tests"`
}

type QuickTest struct {
	ID         string      `yaml:"id" json:"id"`
	Label      string      `yaml:"label" json:"label"`
	Method     string      `yaml:"method" json:"method"`
	URL        string      `yaml:"url" json:"url"`
	Body       interface{} `yaml:"body" json:"body"`
	IsFormData bool        `yaml:"isFormData,omitempty" json:"isFormData,omitempty"`
}
