package docs

import (
	"bytes"
	"flag"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var updateGolden = flag.Bool("update-golden", false, "overwrite golden files with current render output")

// TestRender_Museum renders the Museum example project and compares to a stored
// golden HTML file. The fixture lives under testdata/ so it is decoupled from
// examples/ (which users may freely edit or delete).
// Regenerate the golden with: go test ./pkg/docs -run TestRender -update-golden
func TestRender_Museum(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", "museum.html")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s (%d bytes)", goldenPath, len(got))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v\nhint: run `go test ./pkg/docs -run TestRender -update-golden` to create it", goldenPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("render output differs from golden (len got=%d want=%d).\nhint: if the change is intentional, regenerate with `go test ./pkg/docs -run TestRender -update-golden` and review the diff in git.\nfirst divergence: %s", len(got), len(want), firstDivergence(got, want))
	}
}

// TestRender_StructuralInvariants guards core structural expectations that must
// hold regardless of spec content — ensures we never ship broken HTML.
func TestRender_StructuralInvariants(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)

	invariants := []struct {
		name  string
		check func(string) bool
	}{
		{"starts with doctype", func(s string) bool { return strings.HasPrefix(s, "<!DOCTYPE html>") }},
		{"ends with </html>", func(s string) bool { return strings.HasSuffix(strings.TrimSpace(s), "</html>") }},
		{"has <style>", func(s string) bool { return strings.Contains(s, "<style>") }},
		{"has </style>", func(s string) bool { return strings.Contains(s, "</style>") }},
		{"has plain script", func(s string) bool { return strings.Contains(s, "<script>") }},
		{"self-hosted react",
			func(s string) bool { return strings.Contains(s, "/assets/vendor/react.production.min.js") }},
		{"self-hosted dagre",
			func(s string) bool { return strings.Contains(s, "/assets/vendor/dagre.min.js") }},
		{"no CDN dependency",
			func(s string) bool { return !strings.Contains(s, "unpkg.com") }},
		{"has body", func(s string) bool { return strings.Contains(s, "<body>") && strings.Contains(s, "</body>") }},
		{"no unresolved template tags", func(s string) bool { return !strings.Contains(s, "{{") && !strings.Contains(s, "}}") }},
	}

	for _, inv := range invariants {
		if !inv.check(out) {
			t.Errorf("invariant failed: %s", inv.name)
		}
	}
}

// TestRender_ProjectSwitcher exercises the multi-project dropdown that lives
// in the sidebar header. It must render only when at least two projects are
// loaded, link each entry through `?p=<name>` (with the default project
// linking to the bare prefix), and mark the current project as active.
func TestRender_ProjectSwitcher(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// Default project at root.
	mustWrite("index.yaml", `info:
  title: Portal Default
  version: "1.0.0"
  base_urls: [{ label: Production, url: https://api.example.com, default: true }]
sections:
  - id: home
    title: Home
    description: greeting
    endpoints:
      - { name: Ping, method: GET, path: /ping, auth: none, description: liveness }
`)
	// Two named projects.
	mustWrite("alpha/index.yaml", `info:
  title: Alpha Service
  version: "2.0.0"
  base_urls: [{ label: Production, url: https://alpha.example.com, default: true }]
sections:
  - id: home
    title: Home
    description: greeting
    endpoints:
      - { name: Ping, method: GET, path: /ping, auth: none, description: liveness }
`)
	mustWrite("beta/index.yaml", `info:
  title: Beta Service
  version: "0.3.0"
  base_urls: [{ label: Production, url: https://beta.example.com, default: true }]
sections:
  - id: home
    title: Home
    description: greeting
    endpoints:
      - { name: Ping, method: GET, path: /ping, auth: none, description: liveness }
`)

	h, err := NewHandler(dir, false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	// Mirror what RegisterRoutes does so the switcher links carry a prefix.
	h.prefix = "/docs"

	t.Run("default project marks itself active", func(t *testing.T) {
		out, err := h.Render("")
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		body := string(out)
		if !strings.Contains(body, `<details class="sidebar-project-switcher"`) {
			t.Fatal("switcher missing — dropdown should render with 3 projects loaded")
		}
		// Default project entry: link must be just the prefix, no ?p=.
		if !strings.Contains(body, `href="/docs" class="active"`) {
			t.Errorf("default project link should be `/docs` with active marker; not found")
		}
		// Named projects: link must carry ?p=.
		for _, name := range []string{"alpha", "beta"} {
			needle := `href="/docs?p=` + name + `"`
			if !strings.Contains(body, needle) {
				t.Errorf("expected link %s for project %q, not found", needle, name)
			}
			// And they must NOT be marked active when default is current.
			activeNeedle := `href="/docs?p=` + name + `" class="active"`
			if strings.Contains(body, activeNeedle) {
				t.Errorf("project %q should not be active when rendering default", name)
			}
		}
		// Current label visible at the top of the dropdown.
		if !strings.Contains(body, `<span class="project-switcher-current">default</span>`) {
			t.Errorf("current-project label should read `default`")
		}
	})

	t.Run("alpha selected marks alpha active", func(t *testing.T) {
		out, err := h.Render("alpha")
		if err != nil {
			t.Fatalf("Render alpha: %v", err)
		}
		body := string(out)
		if !strings.Contains(body, `href="/docs?p=alpha" class="active"`) {
			t.Errorf("alpha link should carry active class when rendering alpha")
		}
		if !strings.Contains(body, `<span class="project-switcher-current">alpha</span>`) {
			t.Errorf("current-project label should read `alpha`")
		}
		// Spec content also switched (Alpha title rendered in the main body).
		if !strings.Contains(body, "Alpha Service") {
			t.Errorf("expected Alpha Service title in rendered body")
		}
		// Every download / spec link must preserve the current project.
		// Sidebar Download YAML, footer YAML, footer JSON — all four cases.
		wantLinks := []string{
			`href="/docs/yaml?p=alpha" class="sidebar-download"`,
			`href="/docs/yaml?p=alpha" download`,
			`href="/docs/spec?p=alpha"`,
		}
		for _, want := range wantLinks {
			if !strings.Contains(body, want) {
				t.Errorf("expected link %q to carry ?p=alpha; not found", want)
			}
		}
		// And no stray bare /docs/yaml or /docs/spec without the project.
		badLinks := []string{
			`href="/docs/yaml"`,
			`href="/docs/spec"`,
		}
		for _, bad := range badLinks {
			if strings.Contains(body, bad) {
				t.Errorf("found %q — every yaml/spec link must carry ?p=alpha when alpha is current", bad)
			}
		}
	})
}

// TestRender_DarkMode locks in the dark-mode contract: a toggle button in
// the sidebar, CSS variable overrides under `[data-theme="dark"]`, and an
// inline no-flash bootstrap that sets the theme attribute before any styles
// evaluate. The storage key is part of the contract — changing it would
// silently log every user out of their preference.
func TestRender_DarkMode(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)

	checks := []struct {
		needle string
		why    string
	}{
		{`class="sidebar-theme-toggle"`, "toggle button missing from sidebar"},
		{`'docs_theme'`, "localStorage key must be stable (used by both bootstrap and handler)"},
		{`[data-theme="dark"]`, "dark theme CSS block missing"},
		{`prefers-color-scheme: dark`, "no-flash bootstrap should fall back to system preference"},
		{`setAttribute('data-theme'`, "bootstrap script must set data-theme before styles evaluate"},
	}
	for _, c := range checks {
		if !strings.Contains(body, c.needle) {
			t.Errorf("missing %q — %s", c.needle, c.why)
		}
	}

	// The no-flash script must appear BEFORE the <style> block, otherwise
	// dark-mode users see a flash of light theme on first paint.
	scriptIdx := strings.Index(body, "saved = localStorage.getItem('docs_theme')")
	styleIdx := strings.Index(body, "<style>")
	if scriptIdx == -1 || styleIdx == -1 {
		t.Fatal("could not locate bootstrap script or <style> block")
	}
	if scriptIdx > styleIdx {
		t.Errorf("no-flash bootstrap (offset %d) must come before <style> (offset %d)", scriptIdx, styleIdx)
	}
}

// TestRender_CopyLinkButton checks the copy-link affordance next to Download
// YAML. The button must exist, carry the same href the download link does
// (project-aware), and the inline JS handler must be wired to it.
func TestRender_CopyLinkButton(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)

	if !strings.Contains(body, `class="sidebar-copy-link"`) {
		t.Error("copy-link button missing from sidebar")
	}
	if !strings.Contains(body, `data-copy-href="/yaml"`) {
		t.Error("copy-link button must carry data-copy-href matching the download link (single-project museum spec)")
	}
	if !strings.Contains(body, `navigator.clipboard.writeText`) {
		t.Error("clipboard handler missing — copy button is wired to nothing")
	}
	if !strings.Contains(body, `'.sidebar-copy-link'`) {
		t.Error("delegated click handler selector for copy-link missing")
	}
}

// TestRender_SidebarResizable locks in the user-resizable sidebar contract:
// a drag handle on the right edge that adjusts --sidebar-width and persists
// the chosen width in localStorage. The test asserts the surface markers an
// integrator (or another script) might rely on — handle element, body
// dragging class, storage key, and the bounds the JS clamps to.
func TestRender_SidebarResizable(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)

	checks := []struct {
		needle string
		why    string
	}{
		{`class="sidebar-resize-handle"`, "drag handle DOM node missing"},
		{`'docs_sidebar_width_px'`, "localStorage key must be stable across versions"},
		{`--sidebar-width`, "CSS variable name the JS writes to"},
		{`sidebar-resizing`, "body class that suppresses cursor flicker mid-drag"},
		{`MOBILE_BP = 768`, "mobile breakpoint guard must match @media rule"},
	}
	for _, c := range checks {
		if !strings.Contains(body, c.needle) {
			t.Errorf("missing %q — %s", c.needle, c.why)
		}
	}
}

// TestRender_EndpointDeepLink locks in the shareable deep-link contract:
// stable IDs based on (section, method, name) — not array indices, share
// button per endpoint, hash sync logic in the JS bootstrap, and replaceState
// (not pushState) so the back button isn't polluted by intra-docs navigation.
func TestRender_EndpointDeepLink(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)

	// Stable ID format `endpoint-{section}-{method}-{slug}`. Museum spec has a
	// section id=`museum` with a `Get My Museum` endpoint at GET /api/v1/museum.
	wantPanelID := `id="endpoint-museum-get-get-my-museum"`
	if !strings.Contains(body, wantPanelID) {
		t.Errorf("expected stable panel id %q in rendered HTML — deep links break if IDs depend on array position", wantPanelID)
	}
	// Old indexed format must be gone from endpoint PANELS (tester elements
	// keep indexed IDs because they're internal-only — not URL-shareable).
	if strings.Contains(body, `id="panel-endpoint-`) {
		t.Error("legacy indexed panel IDs must be replaced with stable endpoint anchors")
	}
	// Sidebar nav data-target must match the panel id (otherwise clicking the
	// sidebar lands on nothing).
	if !strings.Contains(body, `data-target="endpoint-museum-get-get-my-museum"`) {
		t.Error("sidebar nav-child-item data-target must use the stable endpoint anchor")
	}
	// Per-endpoint share button (clipboard reuses the existing copy-link
	// handler — see TestRender_CopyLinkButton).
	if !strings.Contains(body, `class="sidebar-copy-link endpoint-share"`) {
		t.Error("per-endpoint share button missing")
	}
	if !strings.Contains(body, `data-copy-href="#endpoint-museum-get-get-my-museum"`) {
		t.Error("share button must carry data-copy-href with the stable hash so the absolute URL is shareable")
	}
	// Hash sync JS — both directions (URL → panel and click → URL).
	for _, marker := range []string{
		"function syncFromHash()",
		`window.addEventListener('hashchange'`,
		"function navigateTo(targetId)",
		"history.replaceState(null, ''",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("missing JS marker %q — deep linking will not work without it", marker)
		}
	}
	// Back button must NOT be polluted: ensure pushState is NOT used for the
	// in-docs navigation (replaceState is the correct choice).
	if strings.Contains(body, "history.pushState(null, '', '#") {
		t.Error("intra-docs navigation should use replaceState, not pushState — pushState pollutes the back button")
	}
}

// TestEndpointAnchor exercises the slug generator directly so the format
// stays stable across versions. Existing shared links rely on it.
func TestEndpointAnchor(t *testing.T) {
	cases := []struct {
		section SectionInfo
		ep      Endpoint
		want    string
	}{
		{SectionInfo{ID: "users"}, Endpoint{Method: "GET", Name: "List Users"}, "endpoint-users-get-list-users"},
		{SectionInfo{ID: "galleries"}, Endpoint{Method: "POST", Name: "Create Gallery"}, "endpoint-galleries-post-create-gallery"},
		{SectionInfo{ID: "users"}, Endpoint{Method: "DELETE", Name: "Delete User"}, "endpoint-users-delete-delete-user"},
		// Case + punctuation normalisation. NB: slug() does not collapse
		// consecutive separator characters — " / " produces three dashes.
		// Documenting current behavior; safe-to-link as URL either way.
		{SectionInfo{ID: "Account-Service"}, Endpoint{Method: "patch", Name: "Reset Password / Send OTP"}, "endpoint-account-service-patch-reset-password---send-otp"},
		// Defensive: empty section id still produces something usable.
		{SectionInfo{ID: ""}, Endpoint{Method: "GET", Name: "Ping"}, "endpoint-get-ping"},
	}
	for _, c := range cases {
		got := endpointAnchor(c.section, c.ep)
		if got != c.want {
			t.Errorf("endpointAnchor(section=%q, method=%q, name=%q) = %q, want %q",
				c.section.ID, c.ep.Method, c.ep.Name, got, c.want)
		}
	}
}

// TestRender_CollapsibleEndpointGroups locks in the contract that section
// groups inside the "🔌 Endpoints" panel render as nested collapsibles
// (default closed, click to expand). Two checks: the DOM markup carries the
// toggle plumbing, and the JS has the walk-up logic that keeps the outer
// panel sized correctly when an inner group expands or collapses.
func TestRender_CollapsibleEndpointGroups(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)

	// Each section in the museum spec should produce a nav-subgroup wrapper
	// with the toggle handler attached.
	if !strings.Contains(body, `class="nav-item nav-subgroup has-children"`) {
		t.Error("section groups must render as `.nav-item.nav-subgroup.has-children` (collapsible)")
	}
	if !strings.Contains(body, `class="nav-item-header nav-subgroup-header" onclick="toggleCollapse(this)"`) {
		t.Error("sub-group header must wire `toggleCollapse(this)` onclick")
	}
	// Default state: NO `open` class on the sub-group — opted into B (closed
	// by default). If a future refactor adds default-open behavior, this
	// guards against that drift.
	if strings.Contains(body, `class="nav-item nav-subgroup has-children open"`) {
		t.Error("sub-groups should NOT default to open — UX choice B (collapsed by default)")
	}
	// Walk-up logic must exist AND be invoked. Count `refreshAncestorMaxHeight`
	// occurrences: 1 = function definition only (the helper is dead code → the
	// regression we're guarding against), 2+ = at least one call site wired up.
	count := strings.Count(body, "refreshAncestorMaxHeight")
	if count < 2 {
		t.Errorf("refreshAncestorMaxHeight appears %d times — expected at least 2 (definition + call). "+
			"Without the call, nested toggles clip and stale dead space remains after collapse.", count)
	}
	if !strings.Contains(body, `.closest('.nav-item.open')`) {
		t.Error("refreshAncestorMaxHeight must walk to ancestor `.nav-item.open` elements")
	}
}

// TestRender_NavChildrenNotClipped guards against a regression where the
// sidebar's expanding nav-children panel had a hard `max-height: 2000px` cap.
// Once a section had ~50+ endpoints, the combined height exceeded the cap and
// `overflow: hidden` silently clipped the bottom rows. Fix: toggleCollapse
// computes `scrollHeight` at click time and sets it inline, so the panel
// always grows to fit.
func TestRender_NavChildrenNotClipped(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "max-height: 2000px") {
		t.Error("regression: hard `max-height: 2000px` cap on nav-children — long endpoint lists will be clipped")
	}
	if !strings.Contains(out, "children.scrollHeight") {
		t.Error("toggleCollapse must set inline max-height to scrollHeight on open — without it, the cap regression returns")
	}
}

// TestRender_EmptyAuthModes guards against the runtime crash reported when a
// spec omits api_tester_defaults.auth_modes — `JSON.parse("null").forEach(...)`
// would blow up loadCredentials before the page finished rendering. The render
// must produce defensive guards (`|| []`) and a default-checked radio so the
// in-page tester can mount without throwing.
func TestRender_EmptyAuthModes(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	yaml := `info:
  title: Demo
  version: "1.0"
  base_urls:
    - { label: Local, url: http://localhost:3000, default: true }
sections:
  - id: hello
    title: Hello
    description: greeting
    endpoints:
      - { name: Ping, method: GET, path: /ping, auth: none, description: liveness }
`
	if err := os.WriteFile(specPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	h, err := NewHandler(specPath, false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)

	if !strings.Contains(out, `JSON.parse("null") || []`) {
		t.Errorf("missing `JSON.parse(\"null\") || []` defensive guard — page would crash on load")
	}
	if !strings.Contains(out, `value="none" id="auth-none-0-0" checked`) {
		t.Errorf("Public radio is not default-checked when auth_modes is empty — `:checked.value` would crash")
	}
}

// TestServeYAML_IndexWithSiblings guards against a regression where /yaml
// downloaded only index.yaml when specRoot pointed at an index.yaml file whose
// parent directory held overlay YAMLs. The loader merges those overlays into
// memory, so the download must reflect the merged view too.
func TestServeYAML_IndexWithSiblings(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router, "/docs")

	req := httptest.NewRequest("GET", "/docs/yaml", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200. body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()

	// Content from sibling files must be present in the download. Each probe
	// is a string that lives in an overlay file, not in index.yaml.
	probes := map[string]string{
		"sections/museum.yaml":        "My Museum (Single Museum Pattern)",
		"sections/artifacts.yaml":     "id: artifacts",
		"sections/articles.yaml":      "Articles (CMS)",
		"guides/file_upload.yaml":     "id: file_upload",
		"screens/museum_screens.yaml": "Museum Dashboard",
	}
	for file, needle := range probes {
		if !strings.Contains(body, needle) {
			t.Errorf("download is missing content from %s (needle %q not found)", file, needle)
		}
	}

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/yaml") {
		t.Errorf("Content-Type = %q, want text/yaml…", ct)
	}
}

// TestRender_TesterUsesTextContentForRawBody guards the XSS-hardening of the
// in-browser tester: when an endpoint's response body is non-JSON text, the
// tester must NOT route it through innerHTML — any endpoint returning HTML
// (a generic 500 page, an error template, an intentionally malicious one)
// would otherwise run script in the operator's origin.
func TestRender_TesterUsesTextContentForRawBody(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)

	// The unsafe pattern: innerHTML concat with raw response body.
	bad := `responseEl.innerHTML = '<code>' + data + '</code>'`
	if strings.Contains(out, bad) {
		t.Errorf("tester still uses innerHTML for raw response body — XSS regression\n  found: %s", bad)
	}
	// Same surface for error.message.
	bad2 := `'<code>Error: ' + error.message + '</code>'`
	if strings.Contains(out, bad2) {
		t.Errorf("tester still uses innerHTML for error.message — XSS regression\n  found: %s", bad2)
	}
	// The safe replacement must be present.
	if !strings.Contains(out, `code.textContent = data`) {
		t.Errorf("expected `code.textContent = data` safe assignment to be present")
	}
}

// TestHandler_ConcurrentReloadAndServe stresses the race between the dev-mode
// file watcher (which replaces h.specs) and request handlers reading from it.
// Run with `-race` to exercise the bug — without the RWMutex this test trips
// the race detector instantly.
func TestHandler_ConcurrentReloadAndServe(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router, "/docs")

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			if err := h.ReloadSpec(); err != nil {
				t.Errorf("ReloadSpec: %v", err)
			}
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		for _, path := range []string{"/docs", "/docs/spec", "/docs/specs", "/docs/yaml", "/docs/openapi"} {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code >= 500 {
				t.Errorf("%s -> %d (body: %q)", path, w.Code, w.Body.String())
			}
		}
	}
	<-done
}

// firstDivergence returns a short description of where two byte slices first differ.
func firstDivergence(a, b []byte) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			start := i - 40
			if start < 0 {
				start = 0
			}
			end := i + 40
			if end > len(a) {
				end = len(a)
			}
			return "byte " + itoa(i) + " — got: " + strings.ReplaceAll(string(a[start:end]), "\n", "\\n")
		}
	}
	return "length mismatch (common prefix equal)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
