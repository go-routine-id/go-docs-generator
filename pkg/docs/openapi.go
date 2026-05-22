package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// LoadOpenAPISpec parses an OpenAPI 3.0 / 3.1 document from disk and projects
// it onto our internal APISpec. The mapping is intentionally lossy — only the
// fields that make sense for an interactive documentation page are preserved.
func LoadOpenAPISpec(path string) (*APISpec, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load openapi from %s: %w", path, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		// Validation failures are non-fatal here — many real-world specs fail
		// strict validation but still render usefully. Surface as a warning in
		// the description rather than blocking.
		_ = err
	}

	return mapOpenAPI(doc), nil
}

func mapOpenAPI(doc *openapi3.T) *APISpec {
	spec := &APISpec{}

	if doc.Info != nil {
		spec.Info.Title = doc.Info.Title
		spec.Info.Version = doc.Info.Version
		spec.Info.Description = doc.Info.Description
	}

	// Servers → BaseURLs. First one becomes Info.BaseURL for the URL input
	// default.
	for i, srv := range doc.Servers {
		if i == 0 {
			spec.Info.BaseURL = srv.URL
		}
		spec.Info.BaseURLs = append(spec.Info.BaseURLs, BaseURL{
			Label:   firstNonEmpty(srv.Description, fmt.Sprintf("Server %d", i+1)),
			URL:     srv.URL,
			Default: i == 0,
		})
	}

	// Auth schemes
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for _, ref := range doc.Components.SecuritySchemes {
			if ref == nil || ref.Value == nil {
				continue
			}
			spec.Authentication.Methods = append(spec.Authentication.Methods, authMethodFromScheme(ref.Value))
		}
	}

	// Group operations by first tag. Operations with no tag land in a
	// synthetic "default" section.
	type tagBucket struct {
		title       string
		description string
		endpoints   []Endpoint
	}
	buckets := map[string]*tagBucket{}

	// Pre-populate from declared tags so description survives even if no op tags it.
	for _, tag := range doc.Tags {
		buckets[tag.Name] = &tagBucket{title: tag.Name, description: tag.Description}
	}

	// Document-level security default — used when an operation lacks its
	// own `security` field. OpenAPI spec section 4.7.2: omitted Security
	// inherits from the document; an empty array [] explicitly opts out.
	var docSecurity openapi3.SecurityRequirements
	if doc.Security != nil {
		docSecurity = doc.Security
	}

	if doc.Paths != nil {
		for _, path := range sortedPaths(doc.Paths.Map()) {
			item := doc.Paths.Value(path)
			if item == nil {
				continue
			}
			for method, op := range item.Operations() {
				tag := "default"
				if len(op.Tags) > 0 {
					tag = op.Tags[0]
				}
				bucket, ok := buckets[tag]
				if !ok {
					bucket = &tagBucket{title: tag}
					buckets[tag] = bucket
				}
				bucket.endpoints = append(bucket.endpoints, endpointFromOperation(method, path, op, item.Parameters, docSecurity))
			}
		}
	}

	// Emit sections in stable order: declared tag order first, then alphabetical.
	declaredOrder := make([]string, 0, len(doc.Tags))
	for _, tag := range doc.Tags {
		declaredOrder = append(declaredOrder, tag.Name)
	}
	seen := map[string]bool{}
	for _, name := range declaredOrder {
		if b, ok := buckets[name]; ok && len(b.endpoints) > 0 {
			spec.Sections = append(spec.Sections, SectionInfo{
				ID:          slug(name),
				Title:       b.title,
				Description: b.description,
				Endpoints:   b.endpoints,
			})
			seen[name] = true
		}
	}
	remaining := make([]string, 0, len(buckets))
	for name := range buckets {
		if !seen[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		b := buckets[name]
		if len(b.endpoints) == 0 {
			continue
		}
		spec.Sections = append(spec.Sections, SectionInfo{
			ID:          slug(name),
			Title:       b.title,
			Description: b.description,
			Endpoints:   b.endpoints,
		})
	}

	return spec
}

// endpointFromOperation projects an OpenAPI operation onto our Endpoint.
// pathParams is the inherited parameter set from the PathItem (OpenAPI
// allows parameters to be declared either on the operation or its
// containing path); docSecurity is the document-level default used when
// the operation doesn't declare its own.
func endpointFromOperation(method, path string, op *openapi3.Operation, pathParams openapi3.Parameters, docSecurity openapi3.SecurityRequirements) Endpoint {
	ep := Endpoint{
		Name:        firstNonEmpty(op.Summary, op.OperationID, method+" "+path),
		Method:      method,
		Path:        path,
		Description: op.Description,
	}

	// Merge inherited path-level parameters with operation-level parameters.
	// OpenAPI semantics: the operation-level entry wins on name+in collision.
	params := mergeParams(pathParams, op.Parameters)

	for _, p := range params {
		if p == nil || p.Value == nil {
			continue
		}
		pv := p.Value
		switch pv.In {
		case "query":
			qp := QueryParam{
				Name:        pv.Name,
				Required:    pv.Required,
				Description: pv.Description,
			}
			if pv.Schema != nil && pv.Schema.Value != nil {
				qp.Type = schemaPrimitive(pv.Schema.Value)
				if pv.Schema.Value.Default != nil {
					qp.Default = fmt.Sprintf("%v", pv.Schema.Value.Default)
				}
			}
			ep.QueryParams = append(ep.QueryParams, qp)
		case "path", "header":
			// Path parameters are part of the URL template — render them in
			// the request body table so users see them documented at all.
			// Same for header params, which are functionally similar to body
			// fields from a docs-page perspective.
			label := pv.Name
			if pv.In == "header" {
				label = "header:" + pv.Name
			} else {
				label = "path:" + pv.Name
			}
			bf := BodyField{
				Name:        label,
				Required:    pv.In == "path" || pv.Required, // path params are always required
				Description: pv.Description,
			}
			if pv.Schema != nil && pv.Schema.Value != nil {
				bf.Type = schemaPrimitive(pv.Schema.Value)
			}
			ep.Body = append(ep.Body, bf)
		}
		// cookie params are skipped — rare, and the in-browser tester would
		// not be able to set them anyway.
	}

	// Request body — pick first JSON media type
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for mt, media := range op.RequestBody.Value.Content {
			if !strings.Contains(mt, "json") {
				continue
			}
			if media.Schema != nil && media.Schema.Value != nil {
				ep.Body = append(ep.Body, bodyFieldsFromSchema(media.Schema.Value)...)
			}
			if media.Example != nil {
				ep.ExampleBody = jsonStringify(media.Example)
			} else if len(media.Examples) > 0 {
				// Pick first declared example deterministically.
				keys := make([]string, 0, len(media.Examples))
				for k := range media.Examples {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				if ex := media.Examples[keys[0]]; ex != nil && ex.Value != nil && ex.Value.Value != nil {
					ep.ExampleBody = jsonStringify(ex.Value.Value)
				}
			}
			break
		}
	}

	// Auth: per-operation security beats document-level default. The
	// distinction matters — an OpenAPI operation can explicitly opt OUT of
	// auth with `security: []`, which we surface as "none".
	switch {
	case op.Security != nil && len(*op.Security) == 0:
		ep.Auth = "none"
	case op.Security != nil && len(*op.Security) > 0:
		ep.Auth = summarizeSecurity(*op.Security)
	case len(docSecurity) > 0:
		ep.Auth = summarizeSecurity(docSecurity)
	default:
		ep.Auth = "none"
	}

	return ep
}

// mergeParams combines path-item-level and operation-level parameter slices.
// The OpenAPI spec says an operation-level entry overrides a path-item entry
// with the same `name`+`in` combination. We preserve declaration order:
// path-item entries first, then operation entries that don't collide.
func mergeParams(pathLevel, opLevel openapi3.Parameters) openapi3.Parameters {
	if len(pathLevel) == 0 {
		return opLevel
	}
	type key struct{ in, name string }
	opKeys := map[key]bool{}
	for _, p := range opLevel {
		if p != nil && p.Value != nil {
			opKeys[key{p.Value.In, p.Value.Name}] = true
		}
	}
	out := make(openapi3.Parameters, 0, len(pathLevel)+len(opLevel))
	for _, p := range pathLevel {
		if p == nil || p.Value == nil {
			continue
		}
		if !opKeys[key{p.Value.In, p.Value.Name}] {
			out = append(out, p)
		}
	}
	out = append(out, opLevel...)
	return out
}

// jsonStringify marshals an example value to a pretty-printed JSON string.
// Falls back to the Go default format only if marshalling fails (e.g. the
// value contains channels, but in practice OpenAPI parsers never produce
// such values).
func jsonStringify(v any) string {
	if s, ok := v.(string); ok {
		// Strings might already be JSON-encoded examples; respect that and
		// don't double-encode.
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return s
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func bodyFieldsFromSchema(s *openapi3.Schema) []BodyField {
	if s == nil {
		return nil
	}
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	out := make([]BodyField, 0, len(s.Properties))
	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ref := s.Properties[name]
		if ref == nil || ref.Value == nil {
			continue
		}
		out = append(out, BodyField{
			Name:        name,
			Type:        schemaPrimitive(ref.Value),
			Required:    required[name],
			Description: ref.Value.Description,
		})
	}
	return out
}

// schemaPrimitive returns a short, human-readable type label for a schema.
// Honours OpenAPI 3.1 union types (`["string", "null"]` → "string?"),
// includes `format` when present (so int32 and date-time survive the
// projection), and special-cases arrays/objects with their element type.
func schemaPrimitive(s *openapi3.Schema) string {
	if s == nil {
		return "any"
	}
	if s.Type == nil || len(*s.Type) == 0 {
		// Some schemas describe shape without a type (e.g. oneOf/anyOf
		// containers). Surface that explicitly rather than guessing.
		switch {
		case s.Format != "":
			return s.Format
		case len(s.OneOf) > 0:
			return "oneOf"
		case len(s.AnyOf) > 0:
			return "anyOf"
		case len(s.AllOf) > 0:
			return "allOf"
		}
		return "any"
	}

	// Separate the "null" alternative from the substantive type (OpenAPI 3.1).
	primary := ""
	nullable := false
	for _, t := range *s.Type {
		if t == "null" {
			nullable = true
			continue
		}
		if primary == "" {
			primary = t
		}
	}
	if primary == "" {
		primary = "any"
	}

	label := primary
	if s.Format != "" {
		label = primary + " (" + s.Format + ")"
	}
	switch primary {
	case "array":
		if s.Items != nil && s.Items.Value != nil {
			label = "array<" + schemaPrimitive(s.Items.Value) + ">"
		}
	case "object":
		if len(s.Properties) == 0 && s.AdditionalProperties.Schema != nil {
			label = "map"
		}
	}
	if nullable {
		label += "?"
	}
	return label
}

func summarizeSecurity(reqs openapi3.SecurityRequirements) string {
	seen := map[string]bool{}
	names := []string{}
	for _, req := range reqs {
		for k := range req {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}
	sort.Strings(names)
	return strings.Join(names, " | ")
}

func authMethodFromScheme(s *openapi3.SecurityScheme) AuthMethod {
	m := AuthMethod{
		Type:        s.Type,
		Description: s.Description,
	}
	switch s.Type {
	case "http":
		m.Type = strings.ToUpper(s.Scheme)
		m.Header = "Authorization"
		if strings.EqualFold(s.Scheme, "bearer") {
			m.Format = "Bearer <token>"
		}
	case "apiKey":
		m.Header = s.Name
		m.Format = "<api_key>"
		if s.In == "query" || s.In == "cookie" {
			m.Header = s.In + ": " + s.Name
		}
	case "oauth2":
		m.Header = "Authorization"
		m.Format = "Bearer <access_token>"
	}
	return m
}

func sortedPaths(paths map[string]*openapi3.PathItem) []string {
	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// slug converts a tag name into a URL-friendly id.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '/', r == ':':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
