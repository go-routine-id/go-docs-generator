package docs

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExportOpenAPI projects an internal APISpec onto an OpenAPI 3.0 document so
// downstream tooling (Postman, Insomnia, Redocly, Stoplight, …) can consume it.
//
// The internal model is narrative-first so the projection cannot be lossless,
// but every documented endpoint surfaces with: request body schema (from
// ep.Body), path parameters extracted from `{x}` placeholders, query
// parameters, per-operation security (so downstream tools render auth
// requirements correctly), and a minimal 200 response.
func ExportOpenAPI(spec *APISpec) *openapi3.T {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       spec.Info.Title,
			Version:     firstNonEmpty(spec.Info.Version, "1.0.0"),
			Description: spec.Info.Description,
		},
		Paths: openapi3.NewPaths(),
	}

	for _, b := range spec.Info.BaseURLs {
		doc.Servers = append(doc.Servers, &openapi3.Server{
			URL:         b.URL,
			Description: b.Label,
		})
	}
	if len(doc.Servers) == 0 && spec.Info.BaseURL != "" {
		doc.Servers = append(doc.Servers, &openapi3.Server{URL: spec.Info.BaseURL})
	}

	// Tags carry section metadata. Use section.ID (stable, URL-safe) as
	// the tag name so the import/export roundtrip preserves grouping.
	tagByID := map[string]string{}
	for _, s := range spec.Sections {
		if s.ID == "" {
			continue
		}
		tagName := s.ID
		tagByID[s.ID] = tagName
		doc.Tags = append(doc.Tags, &openapi3.Tag{Name: tagName, Description: s.Description})
	}

	// Security schemes from AuthenticationInfo.Methods, keyed by a slug of
	// the auth method type. The same key is referenced by each operation's
	// security requirement so downstream OpenAPI consumers (Postman etc.)
	// can render the auth picker correctly.
	schemeNames := map[string]string{} // AuthMethod.Type -> scheme key
	if len(spec.Authentication.Methods) > 0 {
		doc.Components = &openapi3.Components{SecuritySchemes: openapi3.SecuritySchemes{}}
		for i, m := range spec.Authentication.Methods {
			name := nonEmptyID(m.Type, i)
			doc.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: schemeFromAuthMethod(m)}
			schemeNames[strings.ToLower(strings.TrimSpace(m.Type))] = name
		}
	}

	for _, section := range spec.Sections {
		serverOverride := sectionServer(section)
		for _, ep := range section.Endpoints {
			op := buildOperation(ep, section, tagByID, schemeNames)

			item := doc.Paths.Value(ep.Path)
			if item == nil {
				item = &openapi3.PathItem{}
				doc.Paths.Set(ep.Path, item)
			}
			// Per-path server override when the section pins a base_url
			// distinct from the document default.
			if serverOverride != nil && item.Servers == nil {
				item.Servers = openapi3.Servers{serverOverride}
			}
			item.SetOperation(ep.Method, op)
		}
	}

	return doc
}

// buildOperation translates a single Endpoint into a complete OpenAPI
// operation: query/path/header parameters, a JSON request body when
// ep.Body has fields, a minimal but valid 200 response, and the security
// requirement that matches the documented auth label.
func buildOperation(ep Endpoint, section SectionInfo, tagByID map[string]string, schemeNames map[string]string) *openapi3.Operation {
	op := &openapi3.Operation{
		Summary:     ep.Name,
		Description: ep.Description,
	}
	if tagName, ok := tagByID[section.ID]; ok {
		op.Tags = []string{tagName}
	}

	// Query parameters.
	for _, qp := range ep.QueryParams {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{Value: &openapi3.Parameter{
			In:          "query",
			Name:        qp.Name,
			Required:    qp.Required,
			Description: qp.Description,
			Schema:      schemaFromTypeString(qp.Type, qp.Default),
		}})
	}

	// Path parameters: derive from `{name}` placeholders in the path.
	// OpenAPI requires every templated segment to be declared as a path
	// parameter, otherwise lint tools flag the spec as invalid. Honour
	// any explicit `path:<name>` rows in ep.Body for description text.
	pathDescriptions, headerFields, bodyFields := splitBody(ep.Body)
	for _, name := range extractPathParams(ep.Path) {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{Value: &openapi3.Parameter{
			In:          "path",
			Name:        name,
			Required:    true,
			Description: pathDescriptions[name],
			Schema:      schemaFromTypeString("string", ""),
		}})
	}

	// Header parameters.
	for _, h := range headerFields {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{Value: &openapi3.Parameter{
			In:          "header",
			Name:        h.Name,
			Required:    h.Required,
			Description: h.Description,
			Schema:      schemaFromTypeString(h.Type, ""),
		}})
	}

	// Request body.
	if len(bodyFields) > 0 {
		schema := schemaFromBodyFields(bodyFields)
		content := openapi3.Content{
			"application/json": &openapi3.MediaType{Schema: &openapi3.SchemaRef{Value: schema}},
		}
		if ep.ExampleBody != "" {
			var ex any
			if err := json.Unmarshal([]byte(ep.ExampleBody), &ex); err == nil {
				content["application/json"].Example = ex
			} else {
				content["application/json"].Example = ep.ExampleBody
			}
		}
		op.RequestBody = &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{Content: content}}
	}

	// Response — minimal 200 with the example response inlined where present.
	responses := openapi3.NewResponses()
	desc := "OK"
	resp := &openapi3.Response{Description: &desc}
	if ep.ExampleResponse != "" {
		var ex any
		if err := json.Unmarshal([]byte(ep.ExampleResponse), &ex); err == nil {
			resp.Content = openapi3.Content{
				"application/json": &openapi3.MediaType{Example: ex},
			}
		}
	}
	responses.Set("200", &openapi3.ResponseRef{Value: resp})
	op.Responses = responses

	// Per-operation security. "none" is the only explicit opt-out; any
	// other non-empty label that matches a documented auth method
	// produces a security requirement.
	if strings.EqualFold(strings.TrimSpace(ep.Auth), "none") {
		empty := openapi3.SecurityRequirements{}
		op.Security = &empty
	} else if name, ok := schemeNames[strings.ToLower(strings.TrimSpace(ep.Auth))]; ok && ep.Auth != "" {
		sec := openapi3.SecurityRequirements{openapi3.SecurityRequirement{name: []string{}}}
		op.Security = &sec
	}

	return op
}

// sectionServer returns an OpenAPI Server for the section's base_url
// override when one is set; nil otherwise.
func sectionServer(s SectionInfo) *openapi3.Server {
	if s.BaseURL != "" {
		return &openapi3.Server{URL: s.BaseURL}
	}
	for _, b := range s.BaseURLs {
		if b.Default {
			return &openapi3.Server{URL: b.URL, Description: b.Label}
		}
	}
	if len(s.BaseURLs) > 0 {
		return &openapi3.Server{URL: s.BaseURLs[0].URL, Description: s.BaseURLs[0].Label}
	}
	return nil
}

// pathParamRe matches `{name}` segments in an OpenAPI-style path template.
var pathParamRe = regexp.MustCompile(`\{([^{}]+)\}`)

// extractPathParams returns the parameter names from a path template like
// `/users/{id}/orders/{orderId}`. Order matches the order of appearance.
func extractPathParams(path string) []string {
	matches := pathParamRe.FindAllStringSubmatch(path, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// splitBody partitions ep.Body into (path-param descriptions, header
// fields, body fields). The importer encodes path/header parameters in
// ep.Body with prefixed names (`path:id`, `header:X-Foo`); this is the
// inverse, splitting them back out so the exporter can route each kind
// to its correct OpenAPI location.
func splitBody(body []BodyField) (path map[string]string, headers []BodyField, rest []BodyField) {
	path = map[string]string{}
	for _, f := range body {
		switch {
		case strings.HasPrefix(f.Name, "path:"):
			path[strings.TrimPrefix(f.Name, "path:")] = f.Description
		case strings.HasPrefix(f.Name, "header:"):
			headers = append(headers, BodyField{
				Name:        strings.TrimPrefix(f.Name, "header:"),
				Type:        f.Type,
				Required:    f.Required,
				Description: f.Description,
			})
		default:
			rest = append(rest, f)
		}
	}
	return path, headers, rest
}

// schemaFromBodyFields builds an `object` schema with the given fields as
// properties. Empty type strings default to `string` for compatibility
// with downstream tools that require a type.
func schemaFromBodyFields(fields []BodyField) *openapi3.Schema {
	s := &openapi3.Schema{}
	props := openapi3.Schemas{}
	var required []string
	for _, f := range fields {
		props[f.Name] = &openapi3.SchemaRef{Value: &openapi3.Schema{
			Type:        openapiTypeFromLabel(f.Type),
			Description: f.Description,
		}}
		if f.Required {
			required = append(required, f.Name)
		}
	}
	s.Type = &openapi3.Types{"object"}
	s.Properties = props
	s.Required = required
	return s
}

// schemaFromTypeString builds an inline schema for a query/path parameter
// from the docs-generator type label and optional default.
func schemaFromTypeString(label, def string) *openapi3.SchemaRef {
	schema := &openapi3.Schema{Type: openapiTypeFromLabel(label)}
	if def != "" {
		schema.Default = def
	}
	return &openapi3.SchemaRef{Value: schema}
}

// openapiTypeFromLabel maps our human-friendly type strings back to
// OpenAPI primitive type names. Unknown labels default to "string" —
// reasonable since most undocumented types are stringly-typed in practice.
func openapiTypeFromLabel(label string) *openapi3.Types {
	t := strings.ToLower(strings.TrimSpace(label))
	// Strip nullable marker / format adornments produced by schemaPrimitive.
	t = strings.TrimSuffix(t, "?")
	if i := strings.Index(t, " "); i > 0 {
		t = t[:i]
	}
	switch t {
	case "string", "":
		return &openapi3.Types{"string"}
	case "integer", "int", "int32", "int64":
		return &openapi3.Types{"integer"}
	case "number", "float", "double":
		return &openapi3.Types{"number"}
	case "boolean", "bool":
		return &openapi3.Types{"boolean"}
	case "array":
		return &openapi3.Types{"array"}
	case "object", "map":
		return &openapi3.Types{"object"}
	default:
		return &openapi3.Types{"string"}
	}
}

func schemeFromAuthMethod(m AuthMethod) *openapi3.SecurityScheme {
	s := &openapi3.SecurityScheme{Description: m.Description}
	switch {
	case isBearer(m):
		s.Type = "http"
		s.Scheme = "bearer"
	case m.Header != "":
		s.Type = "apiKey"
		s.In = "header"
		s.Name = m.Header
	default:
		// No header declared and not a Bearer family — fall back to bearer
		// so the export validates, but it's an incomplete model.
		s.Type = "http"
		s.Scheme = "bearer"
	}
	return s
}

// isBearer recognises JWT/Bearer-family auth methods. The previous
// version checked only the Format string; that misclassified a
// hand-written `{type: JWT, header: Authorization}` (no Format) as apiKey,
// because the case fell through to the Header check. The fix is to also
// accept Type strings that name the family.
func isBearer(m AuthMethod) bool {
	t := strings.ToLower(m.Type)
	return strings.Contains(m.Format, "Bearer") ||
		strings.Contains(t, "jwt") ||
		strings.Contains(t, "bearer") ||
		strings.Contains(t, "oauth")
}

func nonEmptyID(typeName string, idx int) string {
	if typeName != "" {
		return slug(typeName)
	}
	return "scheme-" + strconv.Itoa(idx)
}
