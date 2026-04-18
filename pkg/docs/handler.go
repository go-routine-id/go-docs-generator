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

// isDirMode returns true if spec is loaded from a directory
func (h *Handler) isDirMode() bool {
	info, err := os.Stat(h.specRoot)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReloadSpec reloads all specs from disk
func (h *Handler) ReloadSpec() error {
	return h.loadAllSpecs()
}

// Render produces the HTML documentation for a given project (empty string = default).
// Exposed for testability (golden tests) and reuse outside HTTP context.
func (h *Handler) Render(project string) ([]byte, error) {
	spec := h.getSpec(project)
	var buf bytes.Buffer
	if err := h.template.ExecuteTemplate(&buf, "docs.gohtml", spec); err != nil {
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
	project := c.Query("p")

	// In single file mode, serve the file directly
	if !h.isDirMode() {
		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=api-spec.yaml")
		c.File(h.specRoot)
		return
	}

	// In dir mode, serve index.yaml for the project
	yamlPath := filepath.Join(h.specRoot, "index.yaml")
	if project != "" {
		yamlPath = filepath.Join(h.specRoot, project, "index.yaml")
	}

	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=api-spec.yaml")
	c.File(yamlPath)
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

// inlineFmt handles inline formatting: **bold**, *italic*, `code`
func inlineFmt(s string) string {
	// Escape HTML
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	// Bold: **text**
	s = replacePair(s, "**", "<strong>", "</strong>")
	// Italic: *text*
	s = replacePair(s, "*", "<em>", "</em>")
	// Inline code: `text`
	s = replacePair(s, "`", "<code>", "</code>")

	return s
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
