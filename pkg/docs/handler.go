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
	"strconv"
	"strings"
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

// Handler handles documentation requests
type Handler struct {
	specRoot string              // path to spec file or directory
	devMode  bool
	specs    map[string]*APISpec // key = project name, "" = default
	template *template.Template
	prefix   string // URL prefix under which routes are registered (e.g. "/docs")
}

// NewHandler creates a new documentation handler
func NewHandler(specPath string, devMode bool) (*Handler, error) {
	h := &Handler{
		specRoot: specPath,
		devMode:  devMode,
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
			// Escape string for JavaScript - replace newlines and quotes
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "'", "\\'")
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", "")
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

// loadAllSpecs loads specs based on whether specRoot is file or directory
func (h *Handler) loadAllSpecs() error {
	info, err := os.Stat(h.specRoot)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", h.specRoot, err)
	}

	if !info.IsDir() {
		// File mode — use loadSpecFromPath (handles index.yaml auto-include)
		spec, err := loadSpecFromPath(h.specRoot)
		if err != nil {
			return err
		}
		h.specs = map[string]*APISpec{"": spec}
		return nil
	}

	// Directory mode — discover all projects
	specs, err := discoverProjects(h.specRoot)
	if err != nil {
		return err
	}
	h.specs = specs
	return nil
}

// getSpec returns the spec for a given project name, or default
func (h *Handler) getSpec(project string) *APISpec {
	if spec, ok := h.specs[project]; ok {
		return spec
	}
	return h.specs[""]
}

// getProjectNames returns a sorted list of project names (excluding default)
func (h *Handler) getProjectNames() []string {
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
// directory name.
func (h *Handler) projectListForSwitcher() []projectInfo {
	if len(h.specs) < 2 {
		return nil
	}
	names := make([]string, 0, len(h.specs))
	for name := range h.specs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]projectInfo, 0, len(names))
	for _, name := range names {
		spec := h.specs[name]
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

// isDirMode returns true if spec is loaded from a directory
func (h *Handler) isDirMode() bool {
	info, err := os.Stat(h.specRoot)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isMergedMode reports whether the in-memory spec was produced by merging
// multiple source files. True when specRoot is a directory, or when specRoot
// is an index.yaml with at least one sibling YAML file (the loader's
// auto-include trigger). In that case the raw bytes of specRoot alone would
// under-represent the spec, so /yaml must serve the marshalled merged view.
func (h *Handler) isMergedMode() bool {
	if h.isDirMode() {
		return true
	}
	if !strings.EqualFold(filepath.Base(h.specRoot), "index.yaml") {
		return false
	}
	parent := filepath.Dir(h.specRoot)
	hasSibling := false
	filepath.WalkDir(parent, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == h.specRoot {
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
	// In dev mode, reload specs on each request
	if h.devMode {
		if err := h.ReloadSpec(); err != nil {
			c.String(http.StatusInternalServerError, "Failed to reload spec: "+err.Error())
			return
		}
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
	// Reload specs to ensure latest version
	if err := h.ReloadSpec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	project := c.Query("p")
	c.JSON(http.StatusOK, h.getSpec(project))
}

// ServeYAML serves the raw YAML spec
func (h *Handler) ServeYAML(c *gin.Context) {
	if err := h.ReloadSpec(); err != nil {
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
	if err := h.ReloadSpec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	projects := make([]projectInfo, 0, len(h.specs))
	for name, spec := range h.specs {
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

// RegisterRoutes registers the documentation routes with a custom prefix
func (h *Handler) RegisterRoutes(router *gin.Engine, prefix string) {
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
	// Expose the prefix so the template can build asset URLs at render time.
	h.prefix = prefix
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
	if err := h.ReloadSpec(); err != nil {
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
// Links are resolved FIRST and parked behind opaque placeholders before the
// emphasis passes run. Without this, the bold/italic/code passes walk the whole
// string — including an already-emitted `<a href="…">` — so a URL containing a
// '*' or '`' (e.g. `/guide*v2*final`) would get tags injected mid-attribute,
// producing a broken link and invalid HTML. The link *text* is still formatted
// (via emphasis), so `[**important**](/x)` renders bold.
func inlineFmt(s string) string {
	// Escape HTML first — neutralises any literal '<', '>' the user wrote.
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	// Markdown links: [text](url). Reject schemes that can execute script
	// (javascript:, data:) — those fall through as literal `[text](url)`.
	// Accepted links become placeholders (which carry no '*' or '`', so the
	// emphasis passes skip over them) and are restored verbatim at the end.
	var links []string
	s = mdLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := mdLinkRe.FindStringSubmatch(m)
		text, url := parts[1], parts[2]
		if !isSafeURL(url) {
			return m
		}
		// HTML-escape happened above, so `&` is already `&amp;`. Only the
		// double-quote needs attribute-escaping for safe insertion into href.
		url = strings.ReplaceAll(url, `"`, "&quot;")
		anchor := `<a href="` + url + `">` + emphasis(text) + `</a>`
		links = append(links, anchor)
		return linkPlaceholder(len(links) - 1)
	})

	s = emphasis(s)

	for i, anchor := range links {
		s = strings.Replace(s, linkPlaceholder(i), anchor, 1)
	}
	return s
}

// emphasis applies the inline emphasis passes: bold, italic, and code. The
// combined `***both***` form is handled before `**`/`*` so a leftover lone '*'
// can never be mis-paired into crossed tags. Run this only on text free of
// link placeholders.
func emphasis(s string) string {
	// Bold+italic: ***text***
	s = replacePair(s, "***", "<strong><em>", "</em></strong>")
	// Bold: **text**
	s = replacePair(s, "**", "<strong>", "</strong>")
	// Italic: *text*
	s = replacePair(s, "*", "<em>", "</em>")
	// Inline code: `text`
	s = replacePair(s, "`", "<code>", "</code>")
	return s
}

// linkPlaceholder builds the sentinel that parks a rendered anchor while the
// emphasis passes run. It deliberately contains no '*' or '`' (nor any other
// emphasis delimiter) so those passes leave it untouched. The NUL bytes make
// accidental collision with real spec prose effectively impossible.
func linkPlaceholder(i int) string {
	return "\x00link:" + strconv.Itoa(i) + "\x00"
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

// replacePair replaces paired delimiters like **text** → <strong>text</strong>,
// scanning left-to-right. Adjacent delimiters with no content between them
// (e.g. a literal `**` in prose) are left verbatim rather than collapsed into
// an empty `<em></em>`/`<strong></strong>` — the empty-inner case is skipped
// past so an unbalanced or decorative delimiter never emits a hollow tag.
func replacePair(s, delim, open, close string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, delim)
		if start == -1 {
			b.WriteString(s)
			break
		}
		after := start + len(delim)
		end := strings.Index(s[after:], delim)
		if end == -1 {
			b.WriteString(s)
			break
		}
		if end == 0 {
			// Adjacent delimiters: emit them literally and resume scanning
			// after the pair so we never produce an empty tag.
			b.WriteString(s[:after+len(delim)])
			s = s[after+len(delim):]
			continue
		}
		b.WriteString(s[:start])
		b.WriteString(open)
		b.WriteString(s[after : after+end])
		b.WriteString(close)
		s = s[after+end+len(delim):]
	}
	return b.String()
}
