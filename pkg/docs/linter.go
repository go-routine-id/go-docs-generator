package docs

import (
	"fmt"
	"sort"
	"strings"
)

// Severity tells the caller whether a diagnostic should block the build.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is a single lint finding.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"` // dotted path into the spec, e.g. ".sections[2].endpoints[0]"
	Message  string   `json:"message"`
}

func (d Diagnostic) String() string {
	tag := "⚠"
	if d.Severity == SeverityError {
		tag = "✖"
	}
	return fmt.Sprintf("%s %-7s %s: %s", tag, d.Severity, d.Path, d.Message)
}

// Lint runs the semantic checks — things the JSON Schema cannot express:
// duplicate IDs, broken cross-references, orphan permission references,
// inconsistent auth labels, and missing descriptions.
//
// The function returns diagnostics in stable order (errors first, then warnings,
// then by path).
func Lint(spec *APISpec) []Diagnostic {
	var out []Diagnostic

	out = append(out, checkRequiredFields(spec)...)
	out = append(out, checkDuplicateIDs(spec)...)
	out = append(out, checkDuplicateEndpoints(spec)...)
	out = append(out, checkFlowAnchorRefs(spec)...)
	out = append(out, checkPermissionRefs(spec)...)
	out = append(out, checkAuthLabelConsistency(spec)...)
	out = append(out, checkAuthModesPresent(spec)...)
	out = append(out, checkFlowDiagramEdgeRefs(spec)...)
	out = append(out, checkScreenCallRefs(spec)...)
	out = append(out, checkEmptyDescriptions(spec)...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity == SeverityError // errors first
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// HasErrors returns true if any diagnostic is severity=error.
func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// checkRequiredFields flags fields that are required for the doc to render
// meaningfully. The JSON Schema marks most fields optional (needed for
// multi-file overlays where each file is partial), so the strict "must have"
// checks live here.
func checkRequiredFields(spec *APISpec) []Diagnostic {
	var out []Diagnostic

	if strings.TrimSpace(spec.Info.Title) == "" {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Path:     ".info.title",
			Message:  "info.title is required for a usable doc page",
		})
	}

	for si, sec := range spec.Sections {
		if sec.ID == "" {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".sections[%d].id", si),
				Message:  "section must have an id",
			})
		}
		if sec.Title == "" {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".sections[%d].title", si),
				Message:  "section must have a title",
			})
		}
		for ei, ep := range sec.Endpoints {
			if ep.Method == "" {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d].method", si, ei),
					Message:  "endpoint must have a method",
				})
			}
			if ep.Path == "" {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d].path", si, ei),
					Message:  "endpoint must have a path",
				})
			}
			if ep.Name == "" {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d].name", si, ei),
					Message:  "endpoint must have a name",
				})
			}
		}
	}

	for gi, g := range spec.Guides {
		if g.ID == "" {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".guides[%d].id", gi),
				Message:  "guide must have an id",
			})
		}
		if g.Title == "" {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".guides[%d].title", gi),
				Message:  "guide must have a title",
			})
		}
	}

	return out
}

// checkDuplicateIDs flags the same ID appearing more than once within a
// single kind (section, guide, screen, event, permission, flow node).
func checkDuplicateIDs(spec *APISpec) []Diagnostic {
	var out []Diagnostic

	track := func(kind, pathPrefix string, ids []string) {
		seen := map[string]int{}
		for i, id := range ids {
			if id == "" {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Path:     fmt.Sprintf("%s[%d].id", pathPrefix, i),
					Message:  kind + " missing id",
				})
				continue
			}
			if first, ok := seen[id]; ok {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Path:     fmt.Sprintf("%s[%d].id", pathPrefix, i),
					Message:  fmt.Sprintf("duplicate %s id %q (also at %s[%d])", kind, id, pathPrefix, first),
				})
				continue
			}
			seen[id] = i
		}
	}

	secIDs := make([]string, len(spec.Sections))
	for i, s := range spec.Sections {
		secIDs[i] = s.ID
	}
	track("section", ".sections", secIDs)

	guideIDs := make([]string, len(spec.Guides))
	for i, g := range spec.Guides {
		guideIDs[i] = g.ID
	}
	track("guide", ".guides", guideIDs)

	screenIDs := make([]string, len(spec.Screens))
	for i, s := range spec.Screens {
		screenIDs[i] = s.ID
	}
	track("screen", ".screens", screenIDs)

	eventIDs := make([]string, len(spec.Events))
	for i, e := range spec.Events {
		eventIDs[i] = e.ID
	}
	track("event", ".events", eventIDs)

	nodeIDs := make([]string, len(spec.FlowDiagramNodes))
	for i, n := range spec.FlowDiagramNodes {
		nodeIDs[i] = n.ID
	}
	track("flow_diagram_node", ".flow_diagram_nodes", nodeIDs)

	permNames := make([]string, len(spec.Permissions))
	for i, p := range spec.Permissions {
		permNames[i] = p.Name
	}
	track("permission", ".permissions", permNames)

	return out
}

// checkFlowAnchorRefs validates that every `#anchor` in guide flow actions
// points to a known endpoint or section id in the document.
func checkFlowAnchorRefs(spec *APISpec) []Diagnostic {
	anchors := collectAnchors(spec)
	var out []Diagnostic
	for gi, g := range spec.Guides {
		for si, step := range g.Flow {
			for ai, action := range step.Actions {
				target := strings.TrimPrefix(action.Endpoint, "#")
				if target == "" || target == action.Endpoint {
					continue // not an anchor reference
				}
				if !anchors[target] {
					out = append(out, Diagnostic{
						Severity: SeverityError,
						Path:     fmt.Sprintf(".guides[%d].flow[%d].actions[%d].endpoint", gi, si, ai),
						Message:  fmt.Sprintf("dangling anchor %q — no section/endpoint with that id", "#"+target),
					})
				}
			}
		}
	}
	return out
}

// collectAnchors returns the set of ids that flow `#anchor` references may
// legitimately point at.
func collectAnchors(spec *APISpec) map[string]bool {
	a := map[string]bool{}
	for _, s := range spec.Sections {
		if s.ID != "" {
			a[s.ID] = true
		}
		for _, ep := range s.Endpoints {
			// Endpoints are commonly referenced by a kebab-cased slug of their name.
			if id := slug(ep.Name); id != "" {
				a[id] = true
			}
		}
	}
	for _, g := range spec.Guides {
		if g.ID != "" {
			a[g.ID] = true
		}
	}
	for _, s := range spec.Screens {
		if s.ID != "" {
			a[s.ID] = true
		}
	}
	for _, e := range spec.Events {
		if e.ID != "" {
			a[e.ID] = true
		}
	}
	return a
}

// checkPermissionRefs warns when an endpoint claims a permission that is not
// documented in the top-level `permissions:` catalog.
func checkPermissionRefs(spec *APISpec) []Diagnostic {
	if len(spec.Permissions) == 0 {
		return nil // no catalog to cross-check against
	}
	known := map[string]bool{}
	for _, p := range spec.Permissions {
		known[p.Name] = true
	}
	var out []Diagnostic
	for si, sec := range spec.Sections {
		for ei, ep := range sec.Endpoints {
			if ep.Permission == "" {
				continue
			}
			if !known[ep.Permission] {
				out = append(out, Diagnostic{
					Severity: SeverityWarning,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d].permission", si, ei),
					Message:  fmt.Sprintf("permission %q is not listed in top-level `permissions:`", ep.Permission),
				})
			}
		}
	}
	return out
}

// checkAuthLabelConsistency warns when the same logical auth method is spelled
// differently across endpoints (e.g. "JWT" vs "jwt" vs "Bearer JWT").
func checkAuthLabelConsistency(spec *APISpec) []Diagnostic {
	// Collect distinct non-empty labels.
	labels := map[string]bool{}
	for _, sec := range spec.Sections {
		for _, ep := range sec.Endpoints {
			if ep.Auth != "" {
				labels[ep.Auth] = true
			}
		}
	}
	if len(labels) < 2 {
		return nil
	}
	// Group by lowercased first token — a heuristic for "same thing, different spelling".
	groups := map[string][]string{}
	for lbl := range labels {
		key := strings.ToLower(firstToken(lbl))
		groups[key] = append(groups[key], lbl)
	}
	var out []Diagnostic
	for key, variants := range groups {
		if len(variants) > 1 {
			sort.Strings(variants)
			out = append(out, Diagnostic{
				Severity: SeverityWarning,
				Path:     ".sections[*].endpoints[*].auth",
				Message:  fmt.Sprintf("inconsistent auth labels for %q-family: %v — pick one spelling", key, variants),
			})
		}
	}
	return out
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

// checkAuthModesPresent fails the lint when an endpoint claims an auth mode
// (anything other than empty or "none") but the API tester has no
// `api_tester_defaults.auth_modes` to back it. Without this check the spec
// passes validation but the rendered tester runs `null.forEach` / `null.find`
// at page load — silent crash for the consumer.
//
// Accumulates ALL failures rather than returning on the first; otherwise a
// spec with thirty endpoints would force the user to re-lint after each fix.
func checkAuthModesPresent(spec *APISpec) []Diagnostic {
	if len(spec.APITesterDefaults.AuthModes) > 0 {
		return nil
	}
	var out []Diagnostic
	for si, sec := range spec.Sections {
		for ei, ep := range sec.Endpoints {
			label := strings.TrimSpace(ep.Auth)
			if label == "" || strings.EqualFold(label, "none") {
				continue
			}
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".sections[%d].endpoints[%d].auth", si, ei),
				Message: fmt.Sprintf("endpoint claims auth %q but api_tester_defaults.auth_modes is empty — "+
					"tester cannot attach credentials and the page will crash on load", label),
			})
		}
	}
	return out
}

// checkDuplicateEndpoints flags repeated (method, path) pairs. A documented
// duplicate is almost always either a copy-paste mistake or an unintended
// section split — the rendered docs would show the same endpoint twice and
// the tester would generate clashing DOM IDs.
func checkDuplicateEndpoints(spec *APISpec) []Diagnostic {
	type pos struct{ s, e int }
	seen := map[string]pos{}
	var out []Diagnostic
	for si, sec := range spec.Sections {
		for ei, ep := range sec.Endpoints {
			if ep.Method == "" || ep.Path == "" {
				continue // already flagged by checkRequiredFields
			}
			key := strings.ToUpper(ep.Method) + " " + ep.Path
			if first, ok := seen[key]; ok {
				out = append(out, Diagnostic{
					Severity: SeverityWarning,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d]", si, ei),
					Message: fmt.Sprintf("duplicate endpoint %q (also defined at .sections[%d].endpoints[%d])",
						key, first.s, first.e),
				})
				continue
			}
			seen[key] = pos{si, ei}
		}
	}
	return out
}

// checkFlowDiagramEdgeRefs flags edges whose source/target points at a node
// id not present in flow_diagram_nodes. ReactFlow tolerates this silently
// (the edge just doesn't render) — left undiagnosed it's a maddening UX bug.
func checkFlowDiagramEdgeRefs(spec *APISpec) []Diagnostic {
	if len(spec.FlowDiagramEdges) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, n := range spec.FlowDiagramNodes {
		if n.ID != "" {
			known[n.ID] = true
		}
	}
	var out []Diagnostic
	for i, e := range spec.FlowDiagramEdges {
		if e.Source != "" && !known[e.Source] {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".flow_diagram_edges[%d].source", i),
				Message:  fmt.Sprintf("edge source %q does not match any flow_diagram_nodes[].id", e.Source),
			})
		}
		if e.Target != "" && !known[e.Target] {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Path:     fmt.Sprintf(".flow_diagram_edges[%d].target", i),
				Message:  fmt.Sprintf("edge target %q does not match any flow_diagram_nodes[].id", e.Target),
			})
		}
	}
	return out
}

// checkScreenCallRefs warns when a screen.calls[] entry references a
// (method, path) pair that isn't a documented endpoint. Caught early it
// prevents docs that promise an API call the backend has no record of.
func checkScreenCallRefs(spec *APISpec) []Diagnostic {
	if len(spec.Screens) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, sec := range spec.Sections {
		for _, ep := range sec.Endpoints {
			if ep.Method != "" && ep.Path != "" {
				known[strings.ToUpper(ep.Method)+" "+ep.Path] = true
			}
		}
	}
	if len(known) == 0 {
		return nil // can't validate if no endpoints declared at all
	}
	var out []Diagnostic
	for si, screen := range spec.Screens {
		for ci, call := range screen.Calls {
			if call.Method == "" || call.Path == "" {
				continue
			}
			key := strings.ToUpper(call.Method) + " " + call.Path
			if !known[key] {
				out = append(out, Diagnostic{
					Severity: SeverityWarning,
					Path:     fmt.Sprintf(".screens[%d].calls[%d]", si, ci),
					Message:  fmt.Sprintf("call %q is not a documented endpoint — typo or undocumented API?", key),
				})
			}
		}
	}
	return out
}

// checkEmptyDescriptions warns for endpoints and sections without a description —
// non-fatal but usually a sign of an unfinished spec.
func checkEmptyDescriptions(spec *APISpec) []Diagnostic {
	var out []Diagnostic
	for si, sec := range spec.Sections {
		if strings.TrimSpace(sec.Description) == "" {
			out = append(out, Diagnostic{
				Severity: SeverityWarning,
				Path:     fmt.Sprintf(".sections[%d]", si),
				Message:  "section has no description",
			})
		}
		for ei, ep := range sec.Endpoints {
			if strings.TrimSpace(ep.Description) == "" {
				out = append(out, Diagnostic{
					Severity: SeverityWarning,
					Path:     fmt.Sprintf(".sections[%d].endpoints[%d]", si, ei),
					Message:  "endpoint has no description",
				})
			}
		}
	}
	return out
}

// LintFile is the path-taking entry point mirroring ValidateFile.
func LintFile(path string) []Diagnostic {
	spec, err := loadSpecFromPath(path)
	if err != nil {
		return []Diagnostic{{Severity: SeverityError, Path: path, Message: err.Error()}}
	}
	return Lint(spec)
}
