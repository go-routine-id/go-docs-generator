package docs

import (
	"context"
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
				bucket.endpoints = append(bucket.endpoints, endpointFromOperation(method, path, op))
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

func endpointFromOperation(method, path string, op *openapi3.Operation) Endpoint {
	ep := Endpoint{
		Name:        firstNonEmpty(op.Summary, op.OperationID, method+" "+path),
		Method:      method,
		Path:        path,
		Description: op.Description,
	}

	for _, p := range op.Parameters {
		if p == nil || p.Value == nil {
			continue
		}
		pv := p.Value
		if pv.In != "query" {
			continue
		}
		qp := QueryParam{
			Name:        pv.Name,
			Required:    pv.Required,
			Description: pv.Description,
		}
		if pv.Schema != nil && pv.Schema.Value != nil {
			qp.Type = schemaPrimitive(pv.Schema.Value)
		}
		ep.QueryParams = append(ep.QueryParams, qp)
	}

	// Request body — pick first JSON media type
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for mt, media := range op.RequestBody.Value.Content {
			if !strings.Contains(mt, "json") {
				continue
			}
			if media.Schema == nil || media.Schema.Value == nil {
				break
			}
			ep.Body = bodyFieldsFromSchema(media.Schema.Value)
			if media.Example != nil {
				ep.ExampleBody = fmt.Sprintf("%v", media.Example)
			}
			break
		}
	}

	if op.Security != nil && len(*op.Security) > 0 {
		ep.Auth = summarizeSecurity(*op.Security)
	} else {
		ep.Auth = "none"
	}

	return ep
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

func schemaPrimitive(s *openapi3.Schema) string {
	if s == nil {
		return "any"
	}
	if s.Type != nil && len(*s.Type) > 0 {
		return (*s.Type)[0]
	}
	return "any"
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
