package docs

import "strings"

// sectionBaseURLs returns the BaseURLs selection the section's API tester
// should offer. Section-level BaseURLs take precedence; otherwise the
// document-wide Info.BaseURLs is used.
func sectionBaseURLs(section SectionInfo, info InfoInfo) []BaseURL {
	if len(section.BaseURLs) > 0 {
		return section.BaseURLs
	}
	return info.BaseURLs
}

// sectionDefaultURL returns the base URL string that should be used as the
// initial "url input" value for an endpoint in this section. Precedence:
//  1. section.BaseURL (explicit single-URL override)
//  2. section.BaseURLs default entry
//  3. section.BaseURLs first entry
//  4. info.BaseURLs default entry
//  5. info.BaseURLs first entry
//  6. info.BaseURL
func sectionDefaultURL(section SectionInfo, info InfoInfo) string {
	if section.BaseURL != "" {
		return section.BaseURL
	}
	if url := defaultOrFirst(section.BaseURLs); url != "" {
		return url
	}
	if url := defaultOrFirst(info.BaseURLs); url != "" {
		return url
	}
	return info.BaseURL
}

// sectionUsesGlobal reports whether a section relies on the document-wide
// base URL (true) or carries its own override (false). The template uses this
// to mark inline selectors so that changing the global environment only
// propagates to sections that inherit.
func sectionUsesGlobal(section SectionInfo) bool {
	return section.BaseURL == "" && len(section.BaseURLs) == 0
}

// testerMethods returns the configured methods for the API tester dropdown,
// falling back to the standard set when the spec does not define any.
func testerMethods(methods []string) []string {
	if len(methods) > 0 {
		return methods
	}
	return []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
}

// testerMethodsWith returns the configured methods plus the endpoint's own
// method if it isn't already in the list. This ensures every dropdown always
// shows the endpoint's actual method as a selectable option.
func testerMethodsWith(methods []string, endpointMethod string) []string {
	base := testerMethods(methods)
	for _, m := range base {
		if strings.EqualFold(m, endpointMethod) {
			return base
		}
	}
	return append(base, strings.ToUpper(endpointMethod))
}

func defaultOrFirst(urls []BaseURL) string {
	for _, b := range urls {
		if b.Default {
			return b.URL
		}
	}
	if len(urls) > 0 {
		return urls[0].URL
	}
	return ""
}


// endpointAnchor returns a stable, shareable element ID for an endpoint panel.
// Format: endpoint-{section-id}-{method-lower}-{slug(name)}. Section scoping
// avoids collisions across sections; including the method disambiguates the
// common REST shape where GET /x and POST /x share a short name like "List".
// Duplicates within (section, method, name) take first-wins — the duplicate-
// endpoint lint surfaces the actual issue.
func endpointAnchor(section SectionInfo, ep Endpoint) string {
	parts := []string{"endpoint"}
	if s := slug(section.ID); s != "" {
		parts = append(parts, s)
	}
	if m := strings.ToLower(strings.TrimSpace(ep.Method)); m != "" {
		parts = append(parts, m)
	}
	if n := slug(ep.Name); n != "" {
		parts = append(parts, n)
	}
	return strings.Join(parts, "-")
}
