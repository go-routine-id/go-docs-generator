package docs

import (
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExportOpenAPI projects an internal APISpec onto an OpenAPI 3.0 document so
// downstream tooling (Postman, Insomnia, Redocly, Stoplight, …) can consume it.
// The mapping is best-effort: our internal model is narrative-first, so many
// OpenAPI niceties (schemas, responses) end up as minimal placeholders.
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

	// Tags carry section metadata.
	for _, s := range spec.Sections {
		if s.Title == "" {
			continue
		}
		doc.Tags = append(doc.Tags, &openapi3.Tag{Name: s.Title, Description: s.Description})
	}

	// Security schemes from AuthenticationInfo.Methods.
	if len(spec.Authentication.Methods) > 0 {
		doc.Components = &openapi3.Components{SecuritySchemes: openapi3.SecuritySchemes{}}
		for i, m := range spec.Authentication.Methods {
			name := nonEmptyID(m.Type, i)
			doc.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: schemeFromAuthMethod(m)}
		}
	}

	for _, section := range spec.Sections {
		for _, ep := range section.Endpoints {
			op := &openapi3.Operation{
				Summary:     ep.Name,
				Description: ep.Description,
			}
			if section.Title != "" {
				op.Tags = []string{section.Title}
			}
			for _, qp := range ep.QueryParams {
				op.Parameters = append(op.Parameters, &openapi3.ParameterRef{Value: &openapi3.Parameter{
					In:          "query",
					Name:        qp.Name,
					Required:    qp.Required,
					Description: qp.Description,
				}})
			}
			// Minimal 200 response so the spec is valid.
			op.Responses = openapi3.NewResponses()

			item := doc.Paths.Value(ep.Path)
			if item == nil {
				item = &openapi3.PathItem{}
				doc.Paths.Set(ep.Path, item)
			}
			item.SetOperation(ep.Method, op)
		}
	}

	return doc
}

func schemeFromAuthMethod(m AuthMethod) *openapi3.SecurityScheme {
	s := &openapi3.SecurityScheme{Description: m.Description}
	switch {
	case m.Header != "" && !isBearer(m):
		s.Type = "apiKey"
		s.In = "header"
		s.Name = m.Header
	default:
		s.Type = "http"
		s.Scheme = "bearer"
	}
	return s
}

func isBearer(m AuthMethod) bool {
	return strings.Contains(m.Format, "Bearer") ||
		strings.Contains(m.Type, "JWT") ||
		strings.Contains(m.Type, "Bearer")
}

func nonEmptyID(typeName string, idx int) string {
	if typeName != "" {
		return slug(typeName)
	}
	return "scheme-" + strconv.Itoa(idx)
}
