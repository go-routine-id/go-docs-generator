// Package docs provides dynamic documentation generation from YAML spec
package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// Handler handles documentation requests
type Handler struct {
	specPath string
	spec     *APISpec
	template *template.Template
	devMode  bool
}

// NewHandler creates a new documentation handler
func NewHandler(specPath string, devMode bool) (*Handler, error) {
	h := &Handler{
		specPath: specPath,
		devMode:  devMode,
	}

	// Load and parse YAML
	if err := h.loadSpec(); err != nil {
		return nil, fmt.Errorf("failed to load spec: %w", err)
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
		"add":           func(a, b int) int { return a + b },
		"md":            mdToHTML,
	}

	tmpl, err := template.New("docs").Funcs(funcMap).Parse(docsTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	h.template = tmpl

	// Start file watcher in dev mode
	if devMode {
		go h.watchSpecFile()
	}

	return h, nil
}

// loadSpec reads and parses the YAML specification from filesystem
func (h *Handler) loadSpec() error {
	data, err := os.ReadFile(h.specPath)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec APISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	h.spec = &spec
	return nil
}

// ReloadSpec reloads the spec from disk (for hot-reload in development)
func (h *Handler) ReloadSpec() error {
	return h.loadSpec()
}

// ServeHTML serves the generated HTML documentation
func (h *Handler) ServeHTML(c *gin.Context) {
	// In dev mode, reload spec on each request
	if h.devMode {
		if err := h.ReloadSpec(); err != nil {
			c.String(http.StatusInternalServerError, "Failed to reload spec: "+err.Error())
			return
		}
	}

	var buf bytes.Buffer
	if err := h.template.Execute(&buf, h.spec); err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate documentation: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buf.String())
}

// ServeSpec serves the API spec as JSON for AI agents
func (h *Handler) ServeSpec(c *gin.Context) {
	// Reload spec to ensure latest version
	if err := h.ReloadSpec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.spec)
}

// ServeYAML serves the raw YAML spec
func (h *Handler) ServeYAML(c *gin.Context) {
	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=api-spec.yaml")
	c.File(h.specPath)
}

// RegisterRoutes registers the documentation routes
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.GET("/docs", h.ServeHTML)
	router.GET("/api/docs/spec", h.ServeSpec)
	router.GET("/api/docs/yaml", h.ServeYAML)
	router.GET("/api/docs/echo", h.ServeEcho)
	router.POST("/api/docs/echo", h.ServeEcho)
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

// watchSpecFile watches the spec file for changes and reloads automatically
func (h *Handler) watchSpecFile() {
	// Simple polling approach - check every 2 seconds
	// For production, consider using fsnotify for event-based watching
	log.Println("🔄 Dev mode: Watching spec file for changes...")

	var lastModTime int64
	for {
		info, err := os.Stat(h.specPath)
		if err != nil {
			log.Printf("⚠️  Failed to stat spec file: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if info.ModTime().Unix() != lastModTime {
			if lastModTime != 0 { // Skip first check
				log.Println("📝 Spec file changed, reloading...")
				if err := h.ReloadSpec(); err != nil {
					log.Printf("❌ Failed to reload spec: %v", err)
				} else {
					log.Println("✅ Spec reloaded successfully")
				}
			}
			lastModTime = info.ModTime().Unix()
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
