// Package docs provides dynamic documentation generation from YAML spec
package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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
		"add": func(a, b int) int { return a + b },
		"md":  mdToHTML,
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
	router.GET(prefix+"/echo", h.ServeEcho)
	router.POST(prefix+"/echo", h.ServeEcho)
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
// Supports: paragraphs, **bold**, *italic*, `code`, - lists, headers.
func mdToHTML(s string) template.HTML {
	s = strings.TrimSpace(s)

	// Split into lines
	lines := strings.Split(s, "\n")
	var result strings.Builder
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Close list if current line is not a list item
		if inList && !strings.HasPrefix(trimmed, "- ") {
			result.WriteString("</ul>")
			inList = false
		}

		if trimmed == "" {
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "### ") {
			result.WriteString("<h3>" + inlineFmt(trimmed[4:]) + "</h3>")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			result.WriteString("<h3>" + inlineFmt(trimmed[3:]) + "</h3>")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			result.WriteString("<h3>" + inlineFmt(trimmed[2:]) + "</h3>")
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") {
			if !inList {
				result.WriteString("<ul style=\"margin:0.5rem 0; padding-left:1.5rem;\">")
				inList = true
			}
			result.WriteString("<li style=\"margin-bottom:0.25rem;\">" + inlineFmt(trimmed[2:]) + "</li>")
			continue
		}

		// Regular paragraph
		result.WriteString("<p style=\"margin-bottom:0.75rem;\">" + inlineFmt(trimmed) + "</p>")
	}

	if inList {
		result.WriteString("</ul>")
	}

	return template.HTML(result.String())
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
