package docs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenAPIRoundtrip exports a docs-generator spec to OpenAPI, re-imports
// it through LoadOpenAPISpec, and asserts the structurally significant
// fields survived. The previous export emitted operations with no request
// body, no path parameters, and the wrong tags — making the roundtrip
// effectively lossy beyond metadata.
func TestOpenAPIRoundtrip(t *testing.T) {
	src := &APISpec{
		Info: InfoInfo{
			Title:       "Roundtrip API",
			Version:     "2.0.0",
			Description: "Test",
			BaseURLs: []BaseURL{
				{Label: "Prod", URL: "https://api.example.com", Default: true},
			},
		},
		Authentication: AuthenticationInfo{
			Methods: []AuthMethod{
				{Type: "Bearer JWT", Header: "Authorization", Format: "Bearer <token>"},
			},
		},
		Sections: []SectionInfo{
			{
				ID:          "users",
				Title:       "Users",
				Description: "User management",
				Endpoints: []Endpoint{
					{
						Name:        "Get user",
						Method:      "GET",
						Path:        "/users/{id}",
						Auth:        "Bearer JWT",
						Description: "Fetch a single user",
					},
					{
						Name:        "Create user",
						Method:      "POST",
						Path:        "/users",
						Auth:        "Bearer JWT",
						Description: "Create a new user",
						Body: []BodyField{
							{Name: "email", Type: "string", Required: true, Description: "Email"},
							{Name: "name", Type: "string", Required: false, Description: "Name"},
						},
						ExampleBody: `{"email":"a@b.com","name":"Alice"}`,
					},
					{
						Name:        "Health",
						Method:      "GET",
						Path:        "/health",
						Auth:        "none",
						Description: "Liveness",
					},
				},
			},
		},
	}

	// Export → JSON → re-parse as OpenAPI doc.
	doc := ExportOpenAPI(src)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	dir := t.TempDir()
	tmp := filepath.Join(dir, "out.json")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatalf("write temp openapi: %v", err)
	}

	got, err := LoadOpenAPISpec(tmp)
	if err != nil {
		t.Fatalf("re-import: %v\nexported doc:\n%s", err, string(data))
	}

	// Info survives.
	if got.Info.Title != src.Info.Title {
		t.Errorf("Info.Title = %q, want %q", got.Info.Title, src.Info.Title)
	}
	if got.Info.Version != src.Info.Version {
		t.Errorf("Info.Version = %q, want %q", got.Info.Version, src.Info.Version)
	}

	// Sections: should re-emerge as one section grouping all three endpoints.
	if len(got.Sections) != 1 {
		t.Fatalf("section count = %d, want 1. sections: %+v", len(got.Sections), got.Sections)
	}
	sec := got.Sections[0]
	if len(sec.Endpoints) != 3 {
		t.Fatalf("endpoint count = %d, want 3. endpoints: %+v", len(sec.Endpoints), sec.Endpoints)
	}

	// Index endpoints by method+path for assertion convenience.
	byKey := map[string]Endpoint{}
	for _, ep := range sec.Endpoints {
		byKey[ep.Method+" "+ep.Path] = ep
	}

	// GET /users/{id} should have a path parameter declared.
	getUser, ok := byKey["GET /users/{id}"]
	if !ok {
		t.Fatalf("missing GET /users/{id} in roundtrip. got endpoints: %+v", sec.Endpoints)
	}
	hasIDPathParam := false
	for _, f := range getUser.Body {
		if strings.HasSuffix(f.Name, "id") && (strings.HasPrefix(f.Name, "path:") || f.Name == "id") {
			hasIDPathParam = true
			break
		}
	}
	if !hasIDPathParam {
		t.Errorf("GET /users/{id} lost path parameter on roundtrip. body: %+v", getUser.Body)
	}

	// POST /users should preserve the request body fields.
	create, ok := byKey["POST /users"]
	if !ok {
		t.Fatalf("missing POST /users")
	}
	wantFields := map[string]bool{"email": true, "name": true}
	for _, f := range create.Body {
		// path/header prefixed fields shouldn't appear for /users (no path params).
		if strings.HasPrefix(f.Name, "path:") || strings.HasPrefix(f.Name, "header:") {
			continue
		}
		delete(wantFields, f.Name)
	}
	if len(wantFields) > 0 {
		t.Errorf("POST /users lost body fields on roundtrip: %v. got body: %+v", wantFields, create.Body)
	}
	// Required flag should survive on `email`.
	for _, f := range create.Body {
		if f.Name == "email" && !f.Required {
			t.Errorf("POST /users.email should be required after roundtrip")
		}
	}
	// Example body should survive.
	if create.ExampleBody == "" {
		t.Errorf("POST /users.example_body was lost on roundtrip")
	} else {
		var got map[string]any
		if err := json.Unmarshal([]byte(create.ExampleBody), &got); err == nil {
			if got["email"] != "a@b.com" {
				t.Errorf("example body email = %v, want a@b.com", got["email"])
			}
		}
	}

	// Auth: explicit "none" survives. Bearer JWT operations should NOT be "none".
	health, ok := byKey["GET /health"]
	if !ok {
		t.Fatalf("missing GET /health")
	}
	if !strings.EqualFold(health.Auth, "none") {
		t.Errorf("GET /health.Auth = %q, want \"none\" (explicit security: [])", health.Auth)
	}
	if strings.EqualFold(getUser.Auth, "none") {
		t.Errorf("GET /users/{id}.Auth should not be \"none\" — original was Bearer JWT, got %q", getUser.Auth)
	}
}

// TestImport_HonoursDocLevelSecurity verifies the previously-broken
// behaviour: when an operation lacks its own `security` field, OpenAPI
// inherits the document-level default. The old importer assigned "none"
// in that case.
func TestImport_HonoursDocLevelSecurity(t *testing.T) {
	yaml := `openapi: 3.0.3
info:
  title: Test
  version: "1"
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /protected:
    get:
      summary: Protected
      responses:
        "200":
          description: OK
  /public:
    get:
      summary: Public
      security: []
      responses:
        "200":
          description: OK
`
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	spec, err := LoadOpenAPISpec(p)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}

	byKey := map[string]Endpoint{}
	for _, sec := range spec.Sections {
		for _, ep := range sec.Endpoints {
			byKey[ep.Method+" "+ep.Path] = ep
		}
	}

	if got := byKey["GET /protected"]; strings.EqualFold(got.Auth, "none") || got.Auth == "" {
		t.Errorf("/protected (inherits doc security) should have non-none auth, got %q", got.Auth)
	}
	if got := byKey["GET /public"]; !strings.EqualFold(got.Auth, "none") {
		t.Errorf("/public (explicit security: []) should be \"none\", got %q", got.Auth)
	}
}

// TestImport_PathAndHeaderParams verifies that path and header parameters
// survive the OpenAPI projection instead of being silently dropped.
func TestImport_PathAndHeaderParams(t *testing.T) {
	yaml := `openapi: 3.0.3
info:
  title: Test
  version: "1"
paths:
  /users/{id}:
    parameters:
      - in: path
        name: id
        required: true
        schema: { type: string }
        description: User ID
    get:
      summary: Get user
      parameters:
        - in: header
          name: X-Trace-Id
          required: false
          schema: { type: string }
          description: Trace
      responses:
        "200":
          description: OK
`
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	spec, err := LoadOpenAPISpec(p)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}

	var ep *Endpoint
	for i, sec := range spec.Sections {
		for j, e := range sec.Endpoints {
			if e.Path == "/users/{id}" {
				ep = &spec.Sections[i].Endpoints[j]
			}
		}
	}
	if ep == nil {
		t.Fatalf("GET /users/{id} not found in import. sections: %+v", spec.Sections)
	}
	gotPath, gotHeader := false, false
	for _, f := range ep.Body {
		if strings.HasPrefix(f.Name, "path:id") {
			gotPath = true
		}
		if strings.HasPrefix(f.Name, "header:X-Trace-Id") {
			gotHeader = true
		}
	}
	if !gotPath {
		t.Errorf("expected path:id parameter in body, got: %+v", ep.Body)
	}
	if !gotHeader {
		t.Errorf("expected header:X-Trace-Id parameter in body, got: %+v", ep.Body)
	}
}

// TestImport_ExampleAsJSON verifies the example fix: previously the
// importer ran fmt.Sprintf("%v", example) on the parsed any value,
// producing Go's map-print syntax `map[k:v]` instead of JSON.
func TestImport_ExampleAsJSON(t *testing.T) {
	yaml := `openapi: 3.0.3
info:
  title: Test
  version: "1"
paths:
  /items:
    post:
      summary: Create
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
            example:
              name: Widget
              quantity: 5
      responses:
        "200":
          description: OK
`
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	spec, err := LoadOpenAPISpec(p)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}

	var ep *Endpoint
	for i, sec := range spec.Sections {
		for j, e := range sec.Endpoints {
			if e.Path == "/items" && e.Method == "POST" {
				ep = &spec.Sections[i].Endpoints[j]
			}
		}
	}
	if ep == nil || ep.ExampleBody == "" {
		t.Fatalf("POST /items example missing. ep=%+v", ep)
	}
	if strings.HasPrefix(ep.ExampleBody, "map[") {
		t.Errorf("example body uses Go map-print syntax instead of JSON: %q", ep.ExampleBody)
	}
	// Must be parseable as JSON.
	var got map[string]any
	if err := json.Unmarshal([]byte(ep.ExampleBody), &got); err != nil {
		t.Errorf("example body is not valid JSON: %v\n  body: %s", err, ep.ExampleBody)
	}
}
