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
		"groupTests": groupTestsByCategory,
		"add":      func(a, b int) int { return a + b },
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

// groupTestsByCategory groups quick tests by their category
func groupTestsByCategory(tests []QuickTest) map[string][]QuickTest {
	groups := map[string][]QuickTest{
		"Museum":    {},
		"Artifacts": {},
		"Images":    {},
	}

	for _, test := range tests {
		switch {
		case strings.Contains(test.ID, "museum") && !strings.Contains(test.ID, "artifact"):
			groups["Museum"] = append(groups["Museum"], test)
		case strings.Contains(test.ID, "artifact"):
			groups["Artifacts"] = append(groups["Artifacts"], test)
		case strings.Contains(test.ID, "image") || strings.Contains(test.ID, "media"):
			groups["Images"] = append(groups["Images"], test)
		default:
			// Add to a default group if doesn't match
			if _, ok := groups["Other"]; !ok {
				groups["Other"] = []QuickTest{}
			}
			groups["Other"] = append(groups["Other"], test)
		}
	}

	// Remove empty groups
	for k, v := range groups {
		if len(v) == 0 {
			delete(groups, k)
		}
	}

	return groups
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
