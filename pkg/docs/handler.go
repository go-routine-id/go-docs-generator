// Package docs provides dynamic documentation generation from YAML spec
package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// yamlUnmarshal is a local alias so the body of handler.go reads naturally;
// keeps the direct dependency on yaml.v3 contained to the import block.
func yamlUnmarshal(data []byte, v any) error { return yaml.Unmarshal(data, v) }

// projectInfo is a lightweight project descriptor for listing
type projectInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	BaseURL     string `json:"base_url"`
}

// Handler handles documentation requests.
//
// Concurrency: specs and the cached isMergedMode flag are guarded by mu.
// The dev-mode file watcher and request handlers BOTH replace h.specs on
// reload, so reads must hold the read lock and writes the write lock.
// template and prefix are set during NewHandler/RegisterRoutes (before any
// concurrent access starts) and treated as immutable thereafter.
type Handler struct {
	specRoot string // path to spec file or directory
	devMode  bool
	template *template.Template
	prefix   string // URL prefix under which routes are registered (e.g. "/docs")

	mu         sync.RWMutex
	specs      map[string]*APISpec // key = project name, "" = default
	mergedMode bool                // cached: true iff specs were produced by merging multiple files
}

// NewHandler creates a new documentation handler. The prefix matches the URL
// prefix the server will mount routes under (e.g. "/docs"); it is baked into
// asset and link URLs emitted by the template. Pass "" for legacy callers
// who set the prefix later via RegisterRoutes — those callers MUST not invoke
// Render before RegisterRoutes runs.
func NewHandler(specPath string, devMode bool) (*Handler, error) {
	return NewHandlerWithPrefix(specPath, devMode, "")
}

// NewHandlerWithPrefix is the prefix-aware constructor. Prefer this over
// NewHandler — pinning the prefix at construction time means Render() can be
// called safely outside the HTTP path (golden tests, snapshot tools).
func NewHandlerWithPrefix(specPath string, devMode bool, prefix string) (*Handler, error) {
	h := &Handler{
		specRoot: specPath,
		devMode:  devMode,
		prefix:   normalizePrefix(prefix),
	}

	// Load specs
	if err := h.loadAllSpecs(); err != nil {
		return nil, fmt.Errorf("failed to load specs: %w", err)
	}

	// Parse template with helper functions
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"join":  strings.Join,
		"json": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"js": func(s string) string {
			// Defense-in-depth: html/template's contextual auto-escaper already
			// JS-escapes string interpolations inside <script>, so the primary
			// concern here is collapsing newlines (which would otherwise break
			// a single-line JS string literal even after html/template's escape
			// pass, because LF/CR are preserved as \n/\r — valid inside JSON
			// strings but invalid inside JS string literals).
			s = strings.ReplaceAll(s, "\r", "")
			s = strings.ReplaceAll(s, "\n", " ")
			// Explicit JS escaping of quote chars + backslash gives us safety
			// even if a future caller forgets the auto-escape boundary
			// (e.g. wraps the value in template.JS).
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "'", "\\'")
			s = strings.ReplaceAll(s, `"`, `\"`)
			// Unicode line separators are valid JSON but break JS string
			// literals. html/template escapes these in <script> contexts but
			// we belt-and-brace.
			s = strings.ReplaceAll(s, " ", "\\u2028")
			s = strings.ReplaceAll(s, " ", "\\u2029")
			return s
		},
		"add":                 func(a, b int) int { return a + b },
		"md":                  mdToHTML,
		"mdi":                 mdInline,
		"sectionBaseURLs":     sectionBaseURLs,
		"sectionDefaultURL":   sectionDefaultURL,
		"sectionUsesGlobal":   sectionUsesGlobal,
		"testerMethods":       testerMethods,
		"testerMethodsWith":   testerMethodsWith,
		"assetURL":            func(file string) string { return h.prefix + "/assets/vendor/" + file },
		"docPath":             func(sub string) string { return h.prefix + "/" + sub },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	h.template = tmpl

	// Start file watcher in dev mode
	if devMode {
		go h.watchSpecFile()
	}

	return h, nil
}

// loadAllSpecs loads specs based on whether specRoot is file or directory.
// Acquires the write lock — callers must NOT already hold mu.
func (h *Handler) loadAllSpecs() error {
	info, err := os.Stat(h.specRoot)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", h.specRoot, err)
	}

	var (
		specs  map[string]*APISpec
		merged bool
	)
	if !info.IsDir() {
		// File mode — use loadSpecFromPath (handles index.yaml auto-include)
		spec, err := loadSpecFromPath(h.specRoot)
		if err != nil {
			return err
		}
		specs = map[string]*APISpec{"": spec}
		merged = computeMergedMode(h.specRoot, false)
	} else {
		// Directory mode — discover all projects
		specs, err = discoverProjects(h.specRoot)
		if err != nil {
			return err
		}
		merged = true
	}

	h.mu.Lock()
	h.specs = specs
	h.mergedMode = merged
	h.mu.Unlock()
	return nil
}

// getSpec returns the spec for a given project name, or default.
// Caller must NOT already hold mu.
func (h *Handler) getSpec(project string) *APISpec {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if spec, ok := h.specs[project]; ok {
		return spec
	}
	return h.specs[""]
}

// snapshotSpecs returns a shallow copy of the specs map for safe iteration
// without holding the lock during downstream work. Each *APISpec is shared —
// callers must treat the contents as read-only.
func (h *Handler) snapshotSpecs() map[string]*APISpec {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]*APISpec, len(h.specs))
	for k, v := range h.specs {
		out[k] = v
	}
	return out
}

// getProjectNames returns a sorted list of project names (excluding default).
// Caller must NOT already hold mu.
func (h *Handler) getProjectNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.specs))
	for name := range h.specs {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// projectListForSwitcher returns the full picker list shown in the sidebar
// dropdown, or nil when fewer than two projects exist (single-project users
// get no chrome). The default project (key "") is included first when it has
// any content, labelled by its info.title; named projects follow in sorted
// order, each carrying its own title for friendlier display than the bare
// directory name. Caller must NOT already hold mu.
func (h *Handler) projectListForSwitcher() []projectInfo {
	specs := h.snapshotSpecs()
	if len(specs) < 2 {
		return nil
	}
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]projectInfo, 0, len(names))
	for _, name := range names {
		spec := specs[name]
		if spec == nil {
			continue
		}
		// Name is the URL key (empty = default project; the switcher template
		// substitutes a friendly label when rendering). Keep it raw so the
		// `?p=<Name>` link round-trips through getSpec correctly.
		out = append(out, projectInfo{
			Name:    name,
			Title:   spec.Info.Title,
			Version: spec.Info.Version,
		})
	}
	return out
}

// isDirMode returns true if spec is loaded from a directory. Used by the
// watcher to decide whether to recurse — does not consult the cached flag
// since it is filesystem state, not spec state.
func (h *Handler) isDirMode() bool {
	info, err := os.Stat(h.specRoot)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isMergedMode reports whether the in-memory spec was produced by merging
// multiple source files. Cached on the handler so ServeYAML doesn't walk the
// filesystem per request. Caller must NOT already hold mu.
func (h *Handler) isMergedMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.mergedMode
}

// computeMergedMode determines whether the spec at the given root would be
// produced via the merge path. True when root is a directory, or when root is
// an index.yaml with at least one sibling YAML file (the loader's auto-include
// trigger). Pulled out of isMergedMode so it can be computed once during load
// and cached.
func computeMergedMode(root string, isDir bool) bool {
	if isDir {
		return true
	}
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		return true
	}
	if !strings.EqualFold(filepath.Base(root), "index.yaml") {
		return false
	}
	parent := filepath.Dir(root)
	hasSibling := false
	_ = filepath.WalkDir(parent, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == root {
			return nil
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
			hasSibling = true
			return filepath.SkipAll
		}
		return nil
	})
	return hasSibling
}

// ReloadSpec reloads all specs from disk
func (h *Handler) ReloadSpec() error {
	return h.loadAllSpecs()
}

// maybeReload triggers a reload only in dev mode. Production servers must
// stay zero-syscall on the hot path; reloads only happen via the file
// watcher (also gated on devMode). Returns the reload error for callers
// that want to surface it.
func (h *Handler) maybeReload() error {
	if !h.devMode {
		return nil
	}
	return h.ReloadSpec()
}

// renderData is the template payload. APISpec is embedded so every existing
// `{{.Info...}}`, `{{.Sections}}`, etc. keeps working unchanged. New fields
// carry render-only context (which projects exist, which one is selected,
// the URL prefix the router is mounted under) without polluting APISpec.
type renderData struct {
	*APISpec
	Projects       []projectInfo // empty when only the default project exists
	CurrentProject string        // "" = default project
	Prefix         string        // e.g. "/docs"
}

// Render produces the HTML documentation for a given project (empty string = default).
// Exposed for testability (golden tests) and reuse outside HTTP context.
func (h *Handler) Render(project string) ([]byte, error) {
	spec := h.getSpec(project)
	data := &renderData{
		APISpec:        spec,
		Projects:       h.projectListForSwitcher(),
		CurrentProject: project,
		Prefix:         h.prefix,
	}
	var buf bytes.Buffer
	if err := h.template.ExecuteTemplate(&buf, "docs.gohtml", data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ServeHTML serves the generated HTML documentation
func (h *Handler) ServeHTML(c *gin.Context) {
	if err := h.maybeReload(); err != nil {
		c.String(http.StatusInternalServerError, "Failed to reload spec: "+err.Error())
		return
	}

	out, err := h.Render(c.Query("p"))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate documentation: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, string(out))
}

// ServeSpec serves the API spec as JSON for AI agents
func (h *Handler) ServeSpec(c *gin.Context) {
	if err := h.maybeReload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	project := c.Query("p")
	c.JSON(http.StatusOK, h.getSpec(project))
}

// ServeYAML serves the raw YAML spec
func (h *Handler) ServeYAML(c *gin.Context) {
	if err := h.maybeReload(); err != nil {
		c.String(http.StatusInternalServerError, "reload spec: "+err.Error())
		return
	}

	project := c.Query("p")

	// Single-file mode: serve the user's original file byte-for-byte so
	// comments, ordering, and their preferred formatting are preserved.
	// Anything else returns the merged effective spec (marshalled), since a
	// partial overlay file on its own would be misleading.
	if !h.isMergedMode() {
		data, err := os.ReadFile(h.specRoot)
		if err != nil {
			c.String(http.StatusInternalServerError, "read spec: "+err.Error())
			return
		}
		c.Header("Content-Disposition", `attachment; filename="spec.yaml"`)
		c.Data(http.StatusOK, "text/yaml; charset=utf-8", data)
		return
	}

	spec := h.getSpec(project)
	if spec == nil {
		c.String(http.StatusNotFound, "project not found: "+project)
		return
	}

	out, err := yaml.Marshal(spec)
	if err != nil {
		c.String(http.StatusInternalServerError, "marshal yaml: "+err.Error())
		return
	}

	// A short header reminds the downloader this is the merged view, not any
	// single source file on disk.
	preamble := "# Merged spec generated by docs-generator.\n" +
		"# This document is the effective YAML behind the rendered page —\n" +
		"# consolidated from every file in the source directory.\n"
	if project != "" {
		preamble += "# project: " + project + "\n"
	}
	preamble += "# yaml-language-server: $schema=" + h.prefix + "/../schemas/spec.schema.json\n\n"

	filename := "spec.yaml"
	if project != "" {
		filename = project + ".yaml"
	}

	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.String(http.StatusOK, preamble+string(out))
}

// ServeProjectList returns available projects
func (h *Handler) ServeProjectList(c *gin.Context) {
	if err := h.maybeReload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	specs := h.snapshotSpecs()
	projects := make([]projectInfo, 0, len(specs))
	for name, spec := range specs {
		if spec == nil {
			continue
		}
		label := name
		if name == "" {
			label = "default"
		}
		projects = append(projects, projectInfo{
			Name:        label,
			Title:       spec.Info.Title,
			Version:     spec.Info.Version,
			Description: spec.Info.Description,
			BaseURL:     spec.Info.BaseURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"count":    len(projects),
	})
}

// RegisterRoutes registers the documentation routes under the given prefix.
// If a prefix was already set via NewHandlerWithPrefix, the argument here
// must either match or be empty — mismatches are a configuration bug and
// would yield routes mounted at one prefix while templates emit links to a
// different one.
func (h *Handler) RegisterRoutes(router *gin.Engine, prefix string) {
	prefix = normalizePrefix(prefix)
	if prefix == "" {
		prefix = h.prefix
	} else if h.prefix != "" && h.prefix != prefix {
		panic(fmt.Sprintf("docs.Handler: prefix mismatch (constructed with %q, RegisterRoutes called with %q)", h.prefix, prefix))
	}
	h.prefix = prefix

	router.GET(prefix, h.ServeHTML)
	router.GET(prefix+"/spec", h.ServeSpec)
	router.GET(prefix+"/specs", h.ServeProjectList)
	router.GET(prefix+"/yaml", h.ServeYAML)
	router.GET(prefix+"/openapi", h.ServeOpenAPI)
	router.POST(prefix+"/validate", h.ServeValidate)
	router.GET(prefix+"/echo", h.ServeEcho)
	router.POST(prefix+"/echo", h.ServeEcho)

	// Self-hosted vendor assets (React, ReactFlow, dagre, ReactFlow CSS).
	// Served from embed.FS so we have zero CDN dependency at runtime.
	router.GET(prefix+"/assets/vendor/:file", h.ServeVendorAsset)
}

// normalizePrefix canonicalises a URL prefix: leading slash, no trailing
// slash, empty stays empty.
func normalizePrefix(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// ServeVendorAsset serves a single file out of the embedded vendor bundle.
// Guards against path traversal by only honouring the :file path param.
func (h *Handler) ServeVendorAsset(c *gin.Context) {
	name := c.Param("file")
	data, err := vendorFS.ReadFile("assets/vendor/" + name)
	if err != nil {
		c.String(http.StatusNotFound, "asset not found")
		return
	}
	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(name, ".js"):
		ct = "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		ct = "text/css; charset=utf-8"
	}
	// Assets are content-addressable-ish (filename fixed per build) so a
	// long cache is safe. Bust by shipping a new binary.
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Data(http.StatusOK, ct, data)
}

// ValidateResponse is the JSON body returned by POST /{prefix}/validate.
// schema_errors come from JSON Schema validation; diagnostics come from the
// semantic linter. `ok` is true only when both lists are empty of errors
// (warnings are allowed).
type ValidateResponse struct {
	OK           bool              `json:"ok"`
	SchemaErrors []ValidationError `json:"schema_errors,omitempty"`
	Diagnostics  []Diagnostic      `json:"diagnostics,omitempty"`
	Summary      ValidateSummary   `json:"summary"`
}

// ValidateSummary counts errors vs warnings for quick client-side branching.
type ValidateSummary struct {
	SchemaErrors  int `json:"schema_errors"`
	LintErrors    int `json:"lint_errors"`
	LintWarnings  int `json:"lint_warnings"`
}

// ServeValidate accepts a spec in the request body, parses it, and returns
// the combined output of the schema validator and the semantic linter as
// JSON. Intended for AI agents and tooling that need to verify a spec
// without running a binary locally.
//
// Body content types accepted:
//   - application/yaml, text/yaml, application/x-yaml, text/plain — raw YAML
//   - application/json — JSON-encoded spec
//
// Size cap: 1 MiB.
func (h *Handler) ServeValidate(c *gin.Context) {
	const maxBody = 1 << 20 // 1 MiB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large or unreadable: " + err.Error()})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty body — send the spec as request body (YAML or JSON)"})
		return
	}

	spec, parseErr := parseSpecBody(raw, c.ContentType())
	if parseErr != nil {
		// Parse failures ARE validation failures — surface them in the usual
		// response shape so clients only need one error path.
		c.JSON(http.StatusOK, ValidateResponse{
			OK:           false,
			SchemaErrors: []ValidationError{{Message: parseErr.Error()}},
			Summary:      ValidateSummary{SchemaErrors: 1},
		})
		return
	}

	schemaErrs := ValidateSpec(spec, "")
	diags := Lint(spec)

	lintErrs, lintWarns := 0, 0
	for _, d := range diags {
		if d.Severity == SeverityError {
			lintErrs++
		} else {
			lintWarns++
		}
	}

	c.JSON(http.StatusOK, ValidateResponse{
		OK:           len(schemaErrs) == 0 && lintErrs == 0,
		SchemaErrors: schemaErrs,
		Diagnostics:  diags,
		Summary: ValidateSummary{
			SchemaErrors: len(schemaErrs),
			LintErrors:   lintErrs,
			LintWarnings: lintWarns,
		},
	})
}

// parseSpecBody converts a YAML or JSON request body into an APISpec.
// Content-Type is consulted as a hint but YAML is tried as a fallback because
// YAML is a JSON superset.
func parseSpecBody(raw []byte, contentType string) (*APISpec, error) {
	if isOpenAPIDocument(raw) {
		// Reject OpenAPI here — we want to validate OUR spec format, not project
		// OpenAPI onto it. Clients can hit /docs/openapi to export instead.
		return nil, fmt.Errorf("body looks like an OpenAPI document — this endpoint validates docs-generator specs only")
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "application/json") {
		var spec APISpec
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("json parse: %w", err)
		}
		return &spec, nil
	}

	// Default: YAML (also parses JSON — yaml.v3 accepts both).
	var spec APISpec
	if err := yamlUnmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	return &spec, nil
}

// ServeOpenAPI exports the current spec as an OpenAPI 3.0 JSON document so
// downstream tools (Postman, Insomnia, Redocly) can consume it.
func (h *Handler) ServeOpenAPI(c *gin.Context) {
	if err := h.maybeReload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	spec := h.getSpec(c.Query("p"))
	c.JSON(http.StatusOK, ExportOpenAPI(spec))
}

// ServeEcho echoes back the received headers and request info for debugging
func (h *Handler) ServeEcho(c *gin.Context) {
	headers := make(map[string]string)
	for name, values := range c.Request.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"method":  c.Request.Method,
		"url":     c.Request.URL.String(),
		"headers": headers,
		"message": "Echo of received request headers",
	})
}

// watchSpecFile watches the spec file/directory for changes and reloads automatically
func (h *Handler) watchSpecFile() {
	slog.Info("dev mode: watching spec for changes", "path", h.specRoot)

	var lastModTime int64
	for {
		var currentMod int64

		if h.isDirMode() {
			filepath.WalkDir(h.specRoot, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if strings.HasSuffix(strings.ToLower(path), ".yaml") || strings.HasSuffix(strings.ToLower(path), ".yml") {
					if info, err := d.Info(); err == nil {
						if t := info.ModTime().Unix(); t > currentMod {
							currentMod = t
						}
					}
				}
				return nil
			})
		} else {
			info, err := os.Stat(h.specRoot)
			if err != nil {
				slog.Warn("failed to stat spec", "path", h.specRoot, "err", err)
				time.Sleep(2 * time.Second)
				continue
			}
			currentMod = info.ModTime().Unix()
		}

		if currentMod != lastModTime {
			if lastModTime != 0 { // Skip first check
				slog.Info("spec changed, reloading")
				if err := h.ReloadSpec(); err != nil {
					slog.Error("failed to reload spec", "err", err)
				} else {
					slog.Info("spec reloaded successfully")
				}
			}
			lastModTime = currentMod
		}

		time.Sleep(2 * time.Second)
	}
}

// mdToHTML converts basic Markdown to template.HTML for safe rendering.
// Supports: paragraphs (CommonMark: blank line = paragraph break, single
// newline = soft wrap within same paragraph), **bold**, *italic*, `code`,
// `- ` lists, and `# ` / `## ` / `### ` headers (all rendered as h3).
func mdToHTML(s string) template.HTML {
	s = strings.TrimSpace(s)

	lines := strings.Split(s, "\n")
	var result strings.Builder
	var para []string // accumulator for consecutive prose lines
	inList := false

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		result.WriteString("<p style=\"margin-bottom:0.75rem;\">" + inlineFmt(strings.Join(para, " ")) + "</p>")
		para = para[:0]
	}
	closeList := func() {
		if inList {
			result.WriteString("</ul>")
			inList = false
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Blank line — paragraph break. Close any open paragraph or list.
		if trimmed == "" {
			flushPara()
			closeList()
			continue
		}

		// Headers terminate the previous block.
		if strings.HasPrefix(trimmed, "### ") {
			flushPara()
			closeList()
			result.WriteString("<h3>" + inlineFmt(trimmed[4:]) + "</h3>")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flushPara()
			closeList()
			result.WriteString("<h3>" + inlineFmt(trimmed[3:]) + "</h3>")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flushPara()
			closeList()
			result.WriteString("<h3>" + inlineFmt(trimmed[2:]) + "</h3>")
			continue
		}

		// List items — flush paragraph first, then open/continue the list.
		if strings.HasPrefix(trimmed, "- ") {
			flushPara()
			if !inList {
				result.WriteString("<ul style=\"margin:0.5rem 0; padding-left:1.5rem;\">")
				inList = true
			}
			result.WriteString("<li style=\"margin-bottom:0.25rem;\">" + inlineFmt(trimmed[2:]) + "</li>")
			continue
		}

		// A prose line AFTER a list implicitly ends the list.
		closeList()

		// Accumulate into current paragraph — flushed on blank line or EOF.
		para = append(para, trimmed)
	}

	flushPara()
	closeList()

	return template.HTML(result.String())
}

// mdInline renders inline-only markdown (bold, italic, code) for contexts
// where a surrounding block element already exists — table cells, <p> tags,
// <strong> wrappers. Unlike mdToHTML, it does NOT emit <p>, <h3>, or <ul>
// tags, so it is safe to use inside any existing inline/block wrapper.
func mdInline(s string) template.HTML {
	return template.HTML(inlineFmt(strings.TrimSpace(s)))
}

// mdLinkRe matches `[text](url)`. The URL stops at the first ')' or whitespace
// — good enough for the project-internal links and external URLs we encounter
// in spec descriptions; exotic URLs containing literal ')' need to be raw.
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

// inlineFmt handles inline formatting: [text](url), **bold**, *italic*, `code`.
//
// Links are extracted into placeholders BEFORE the emphasis passes so that
// characters inside the href (`*`, `` ` ``) are not mistaken for emphasis
// delimiters — without this, `[x](https://a/*b*)` would produce
// `<a href="https://a/<em>b</em>">x</a>`, a broken anchor. The link text is
// formatted separately so `[**bold**](/x)` still renders emphasis.
func inlineFmt(s string) string {
	// Escape HTML first — neutralises any literal '<', '>' the user wrote.
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	// Phase 1: extract links into placeholder tokens. \x00 is a safe sentinel
	// since the HTML-escape pass above stripped angle brackets and entities
	// already encode any user-supplied bytes other than \x00 itself, which
	// has no legitimate place in a docs spec.
	type linkTok struct{ text, href string }
	var links []linkTok
	s = mdLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := mdLinkRe.FindStringSubmatch(m)
		text, url := parts[1], parts[2]
		if !isSafeURL(url) {
			return m // unsafe scheme — leave as literal `[text](url)`
		}
		url = strings.ReplaceAll(url, `"`, "&quot;")
		links = append(links, linkTok{text: text, href: url})
		return fmt.Sprintf("\x00%d\x00", len(links)-1)
	})

	// Phase 2: emphasis + code on the outer string (link placeholders are
	// inert under these passes).
	s = applyEmphasis(s)

	// Phase 3: substitute placeholders with formatted anchors. The link
	// text gets its own emphasis pass so `[**bold**](/x)` still works.
	for i, link := range links {
		formatted := applyEmphasis(link.text)
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00%d\x00", i), `<a href="`+link.href+`">`+formatted+`</a>`)
	}

	return s
}

// applyEmphasis runs the bold / italic / code substitutions. Pulled out so
// the link-text and outer-string passes share the same logic.
func applyEmphasis(s string) string {
	s = replacePair(s, "**", "<strong>", "</strong>")
	s = replacePair(s, "*", "<em>", "</em>")
	s = replacePair(s, "`", "<code>", "</code>")
	return s
}

// isSafeURL whitelists URL schemes that can't execute arbitrary script when
// inserted into an href. Everything outside the list is treated as literal
// text (the [text](url) substitution falls through unchanged).
func isSafeURL(u string) bool {
	lower := strings.ToLower(u)
	switch {
	case strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "mailto:"),
		strings.HasPrefix(lower, "/"),
		strings.HasPrefix(lower, "#"),
		strings.HasPrefix(lower, "./"),
		strings.HasPrefix(lower, "../"):
		return true
	}
	return false
}

// replacePair replaces paired delimiters like **text** → <strong>text</strong>
func replacePair(s, delim, open, close string) string {
	for {
		start := strings.Index(s, delim)
		if start == -1 {
			break
		}
		after := start + len(delim)
		end := strings.Index(s[after:], delim)
		if end == -1 {
			break
		}
		inner := s[after : after+end]
		s = s[:start] + open + inner + close + s[after+end+len(delim):]
	}
	return s
}
