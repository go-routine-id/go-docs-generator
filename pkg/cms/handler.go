package cms

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Server bundles everything a CMS instance needs to serve HTTP. Construct
// with NewServer, then call RegisterRoutes to mount handlers on a gin Engine.
type Server struct {
	store   *Store
	auth    *Authenticator
	specDir string // absolute, cleaned path to the YAML root
	tmpl    *template.Template
}

// NewServer wires the dependencies and parses the embedded templates. specDir
// is resolved to an absolute path so subsequent traversal guards have a stable
// prefix to check against.
func NewServer(store *Store, auth *Authenticator, specDir string) (*Server, error) {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		return nil, fmt.Errorf("resolve spec dir: %w", err)
	}
	tmpl, err := template.New("").ParseFS(TemplateFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{store: store, auth: auth, specDir: abs, tmpl: tmpl}, nil
}

// SpecDir returns the resolved absolute path to the YAML root. Exposed so
// main can log it on startup.
func (s *Server) SpecDir() string { return s.specDir }

// RegisterRoutes wires all CMS endpoints onto r. The auth split is:
//   - /login (GET/POST), /healthz: public
//   - everything else: behind RequireAuth
func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/login", s.handleLoginForm)
	r.POST("/login", s.handleLoginSubmit)

	authed := r.Group("/", s.auth.RequireAuth())
	authed.POST("/logout", s.handleLogout)
	authed.GET("/", s.handleRoot)
	authed.GET("/guides", s.handleGuidesList)
	authed.GET("/guides/edit", s.handleGuideEditForm)
	authed.POST("/guides/edit", s.handleGuideEditSubmit)
}

// render executes one of the embedded templates and writes the result.
// HTML errors are logged but not surfaced — by the time we're rendering the
// response status is already 200 in most paths.
func (s *Server) render(c *gin.Context, name string, data any) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		slog.Error("render template", "name", name, "err", err)
	}
}

// ---- public handlers ----

func (s *Server) handleLoginForm(c *gin.Context) {
	s.render(c, "login", gin.H{"Error": c.Query("error")})
}

func (s *Server) handleLoginSubmit(c *gin.Context) {
	password := c.PostForm("password")
	if !s.auth.CheckPassword(password) {
		s.render(c, "login", gin.H{"Error": "Wrong password."})
		return
	}
	if _, err := s.auth.Login(c); err != nil {
		slog.Error("login mint session", "err", err)
		s.render(c, "login", gin.H{"Error": "Internal error — see server logs."})
		return
	}
	c.Redirect(http.StatusSeeOther, "/guides")
}

// ---- authed handlers ----

func (s *Server) handleLogout(c *gin.Context) {
	_ = s.auth.Logout(c)
	c.Redirect(http.StatusSeeOther, "/login")
}

func (s *Server) handleRoot(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/guides")
}

func (s *Server) handleGuidesList(c *gin.Context) {
	guides, err := DiscoverGuides(s.specDir)
	if err != nil {
		slog.Error("discover guides", "err", err)
		c.String(http.StatusInternalServerError, "list guides: "+err.Error())
		return
	}
	// File paths displayed to editors are relative to the spec dir — full
	// paths are noisy and leak the host layout. They're still passed verbatim
	// in the edit URL (absolute) so the publish path stays unambiguous.
	for i := range guides {
		if rel, err := filepath.Rel(s.specDir, guides[i].FilePath); err == nil {
			guides[i].FilePath = rel
		}
	}
	s.render(c, "guides_list", gin.H{
		"User":    sessionUser(c),
		"SpecDir": s.specDir,
		"Guides":  guides,
		"Flash":   c.Query("flash"),
	})
}

func (s *Server) handleGuideEditForm(c *gin.Context) {
	file, id := c.Query("file"), c.Query("id")
	abs, err := s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	guide, err := LoadGuide(abs, id)
	if errors.Is(err, ErrGuideNotFound) {
		c.String(http.StatusNotFound, "guide not found")
		return
	}
	if err != nil {
		slog.Error("load guide", "file", abs, "id", id, "err", err)
		c.String(http.StatusInternalServerError, "load guide: "+err.Error())
		return
	}
	// Display path is relative; submit path keeps the original (relative) query
	// — we re-resolve on POST.
	guide.FilePath = file
	s.render(c, "guide_edit", gin.H{
		"User":  sessionUser(c),
		"Guide": guide,
		"Error": c.Query("error"),
	})
}

func (s *Server) handleGuideEditSubmit(c *gin.Context) {
	file, id := c.Query("file"), c.Query("id")
	abs, err := s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	update := GuideEntry{
		FilePath:    abs,
		ID:          id,
		Icon:        strings.TrimSpace(c.PostForm("icon")),
		Title:       strings.TrimSpace(c.PostForm("title")),
		Description: c.PostForm("description"),
	}
	if update.Title == "" {
		s.redirectWithQuery(c, "/guides/edit", url.Values{
			"file":  {file},
			"id":    {id},
			"error": {"Title cannot be empty."},
		})
		return
	}
	if err := SaveGuide(abs, update); err != nil {
		slog.Error("save guide", "file", abs, "id", id, "err", err)
		s.redirectWithQuery(c, "/guides/edit", url.Values{
			"file":  {file},
			"id":    {id},
			"error": {"Save failed: " + err.Error()},
		})
		return
	}
	slog.Info("guide published", "file", abs, "id", id, "user", sessionUser(c))
	s.redirectWithQuery(c, "/guides", url.Values{
		"flash": {fmt.Sprintf("Published %s (%s).", id, file)},
	})
}

// ---- helpers ----

// resolveSpecPath turns a user-supplied (relative) path into an absolute one
// after asserting it lives inside specDir. Rejects empty paths, ".." escapes,
// and absolute paths pointing elsewhere — the CMS must never write outside
// its configured root.
func (s *Server) resolveSpecPath(input string) (string, error) {
	if input == "" {
		return "", errors.New("missing file parameter")
	}
	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(s.specDir, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	// filepath.Clean has already normalized; check the prefix to block escapes.
	rootWithSep := s.specDir + string(filepath.Separator)
	if abs != s.specDir && !strings.HasPrefix(abs, rootWithSep) {
		return "", errors.New("path escapes spec dir")
	}
	return abs, nil
}

// redirectWithQuery is a convenience for sending the editor back to a page
// with flash/error params in the URL.
func (s *Server) redirectWithQuery(c *gin.Context, path string, q url.Values) {
	c.Redirect(http.StatusSeeOther, path+"?"+q.Encode())
}

// sessionUser pulls the session user name out of the context for templates.
// Falls back to "admin" so templates never render blank.
func sessionUser(c *gin.Context) string {
	if v, ok := c.Get(ContextSessionKey); ok {
		if s, ok := v.(*Session); ok && s.User != "" {
			return s.User
		}
	}
	return AdminUser
}
