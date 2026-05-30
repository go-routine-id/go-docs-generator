package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
// is resolved to an absolute path AND symlinks are followed so subsequent
// traversal guards compare against the real filesystem root — otherwise a
// symlinked subdirectory inside specDir could let writes escape the sandbox
// (e.g. spec/escape -> /etc would pass the HasPrefix(abs, specDir+sep) check).
func NewServer(store *Store, auth *Authenticator, specDir string) (*Server, error) {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		return nil, fmt.Errorf("resolve spec dir: %w", err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
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
	// Draft + preview flow: edits hit /draft (just persist) or /preview
	// (persist + show diff); /publish applies the saved draft to disk.
	// Publishing without a draft is disallowed so the editor always sees
	// the diff before writing the file.
	authed.POST("/guides/draft", s.handleGuideDraftSave)
	authed.POST("/guides/draft/discard", s.handleGuideDraftDiscard)
	authed.POST("/guides/preview", s.handleGuidePreview)
	authed.POST("/guides/publish", s.handleGuidePublish)
	// Backwards-compat shim: the pre-draft MVP exposed POST /guides/edit
	// as a direct publish. Bookmarks, browser form-resubmits, and the
	// implicit-submit fallback for the edit form would otherwise 404; 307
	// preserves method+body so the POST replays cleanly through the
	// preview flow (which is the safer default — it persists a draft and
	// shows the diff rather than writing the file).
	authed.POST("/guides/edit", func(c *gin.Context) {
		target := "/guides/preview"
		if q := c.Request.URL.RawQuery; q != "" {
			target += "?" + q
		}
		c.Redirect(http.StatusTemporaryRedirect, target)
	})
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
	draftKeys, err := s.draftKeySet()
	if err != nil {
		slog.Warn("list drafts (badges will be wrong)", "err", err)
	}
	// File paths displayed to editors are relative to the spec dir — full
	// paths are noisy and leak the host layout. They're still passed verbatim
	// in the edit URL (absolute) so the publish path stays unambiguous.
	type listRow struct {
		GuideEntry
		HasDraft bool
	}
	rows := make([]listRow, 0, len(guides))
	for _, g := range guides {
		row := listRow{GuideEntry: g, HasDraft: draftKeys[draftKey(g.FilePath, g.ID)]}
		if rel, err := filepath.Rel(s.specDir, g.FilePath); err == nil {
			row.FilePath = rel
		}
		rows = append(rows, row)
	}
	s.render(c, "guides_list", gin.H{
		"User":    sessionUser(c),
		"SpecDir": s.specDir,
		"Guides":  rows,
		"Flash":   c.Query("flash"),
	})
}

// handleGuideEditForm shows the edit form. When a saved draft exists for the
// (file, guide) pair it pre-fills from the draft and surfaces a banner so the
// editor knows what they're editing — otherwise it falls back to the
// published YAML.
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
	// The draft banner is shown for THREE cases so the editor always has a
	// path to inspect/clear an out-of-band draft:
	//   1. Healthy draft present → pre-fill form with draft values
	//   2. Corrupt draft (decode fails) → show error banner + discard button
	//   3. DB error reading drafts → surface the error in the banner
	// Previously any non-ErrDraftNotFound error silently fell through, which
	// left the list-view "📝 draft" badge orphaned with no UI to clear it.
	var draftBanner string
	var hasDraft bool
	d, err := s.store.GetDraft(abs, id)
	switch {
	case errors.Is(err, ErrDraftNotFound):
		// no draft — happy path
	case err != nil:
		hasDraft = true
		draftBanner = "Could not load draft: " + err.Error() + "."
	default:
		hasDraft = true
		drafted, derr := decodeGuidePayload(d.Payload)
		if derr != nil {
			draftBanner = "A draft exists but its payload is corrupt (" + derr.Error() + "). Discard it to start over."
		} else {
			guide.Icon = drafted.Icon
			guide.Title = drafted.Title
			guide.Description = drafted.Description
			draftBanner = "Editing an unsaved draft from " + humanAgo(d.UpdatedAt) + "."
		}
	}
	guide.FilePath = file
	s.render(c, "guide_edit", gin.H{
		"User":        sessionUser(c),
		"Guide":       guide,
		"DraftBanner": draftBanner,
		"HasDraft":    hasDraft,
		"Flash":       c.Query("flash"),
		"Error":       c.Query("error"),
	})
}

// handleGuideDraftSave persists the form values as a draft without touching
// the YAML file. The editor stays on the edit form so they can keep iterating.
func (s *Server) handleGuideDraftSave(c *gin.Context) {
	file, id, update, abs, ok := s.collectEditForm(c)
	if !ok {
		return
	}
	if err := s.persistDraft(abs, id, update); err != nil {
		slog.Error("save draft", "err", err)
		s.editError(c, file, id, "Save draft failed: "+err.Error())
		return
	}
	s.redirectWithQuery(c, "/guides/edit", url.Values{
		"file":  {file},
		"id":    {id},
		"flash": {"Draft saved."},
	})
}

// handleGuideDraftDiscard removes the draft for a guide. The next edit form
// load will fall back to the published YAML. DB errors are surfaced to the
// editor instead of being silently swallowed with a success flash — the
// "Draft discarded." message would otherwise lie to the user when the
// underlying delete failed.
func (s *Server) handleGuideDraftDiscard(c *gin.Context) {
	file, id := c.Query("file"), c.Query("id")
	abs, err := s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.DeleteDraft(abs, id); err != nil {
		slog.Error("delete draft", "err", err)
		s.redirectWithQuery(c, "/guides/edit", url.Values{
			"file":  {file},
			"id":    {id},
			"error": {"Discard failed: " + err.Error()},
		})
		return
	}
	s.redirectWithQuery(c, "/guides/edit", url.Values{
		"file":  {file},
		"id":    {id},
		"flash": {"Draft discarded."},
	})
}

// handleGuidePreview reads the current YAML and computes the proposed YAML
// FIRST, then persists the draft only on success — that way a missing /
// renamed file at preview time aborts cleanly without stranding the editor
// with a saved draft they can't reach (the edit form would also 404 on the
// missing file, leaving the badge with no UI path to discard).
func (s *Server) handleGuidePreview(c *gin.Context) {
	file, id, update, abs, ok := s.collectEditForm(c)
	if !ok {
		return
	}
	current, err := s.fileBytes(abs)
	if err != nil {
		s.editError(c, file, id, "Read current YAML failed: "+err.Error())
		return
	}
	proposed, err := ProposedGuideYAMLFromBytes(abs, current, update)
	if err != nil {
		s.editError(c, file, id, "Render proposed YAML failed: "+err.Error())
		return
	}
	if err := s.persistDraft(abs, id, update); err != nil {
		slog.Error("preview: persist draft", "err", err)
		s.editError(c, file, id, "Save draft failed: "+err.Error())
		return
	}
	lines, stats, err := DiffYAML(current, proposed, file+" (current)", file+" (draft)")
	if err != nil {
		s.editError(c, file, id, "Diff failed: "+err.Error())
		return
	}
	s.render(c, "guide_preview", gin.H{
		"User":      sessionUser(c),
		"File":      file,
		"ID":        id,
		"Title":     update.Title,
		"DiffLines": lines,
		"Stats":     stats,
		"NoChanges": len(lines) == 0,
	})
}

// handleGuidePublish applies the saved draft to disk and clears it. Publishing
// requires a draft (the edit form's Publish path goes through preview first),
// so a missing draft is treated as "already published, just redirect home".
func (s *Server) handleGuidePublish(c *gin.Context) {
	file, id := c.Query("file"), c.Query("id")
	abs, err := s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	d, err := s.store.GetDraft(abs, id)
	if errors.Is(err, ErrDraftNotFound) {
		s.redirectWithQuery(c, "/guides", url.Values{
			"flash": {"Nothing to publish — no draft for this guide."},
		})
		return
	}
	if err != nil {
		slog.Error("publish: load draft", "err", err)
		c.String(http.StatusInternalServerError, "load draft: "+err.Error())
		return
	}
	update, err := decodeGuidePayload(d.Payload)
	if err != nil {
		slog.Error("publish: decode draft", "err", err)
		s.editError(c, file, id, "Decode draft failed: "+err.Error())
		return
	}
	update.FilePath = abs
	update.ID = id
	// Re-validate Title here — collectEditForm's check only runs when the
	// editor posts the form. A draft persisted by a previous build, a manual
	// DB edit, or a future migration could leave the row with an empty Title;
	// publishing it as-is would silently break the invariant the edit form
	// is supposed to enforce.
	if strings.TrimSpace(update.Title) == "" {
		s.editError(c, file, id, "Cannot publish: draft has an empty Title. Open the edit form and provide one.")
		return
	}
	if err := SaveGuide(abs, update); err != nil {
		slog.Error("publish: save guide", "err", err)
		s.editError(c, file, id, "Publish failed: "+err.Error())
		return
	}
	if err := s.store.DeleteDraft(abs, id); err != nil {
		slog.Warn("publish: delete draft (state may be stale)", "err", err)
	}
	slog.Info("guide published", "file", abs, "id", id, "user", sessionUser(c))
	s.redirectWithQuery(c, "/guides", url.Values{
		"flash": {fmt.Sprintf("Published %s (%s).", id, file)},
	})
}

// collectEditForm pulls the editable fields out of a POST and resolves the
// path. Centralised so /draft, /preview, and /publish-flow all agree on what
// the form contains (and so all share the same Title-required guard). Returns
// the relative file (for redirects) and absolute file (for IO + DB).
func (s *Server) collectEditForm(c *gin.Context) (file, id string, update GuideEntry, abs string, ok bool) {
	file = c.Query("file")
	id = c.Query("id")
	var err error
	abs, err = s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return file, id, update, "", false
	}
	update = GuideEntry{
		FilePath:    abs,
		ID:          id,
		Icon:        strings.TrimSpace(c.PostForm("icon")),
		Title:       strings.TrimSpace(c.PostForm("title")),
		Description: c.PostForm("description"),
	}
	if update.Title == "" {
		s.editError(c, file, id, "Title cannot be empty.")
		return file, id, update, "", false
	}
	return file, id, update, abs, true
}

// persistDraft JSON-encodes update and upserts it into the drafts table.
func (s *Server) persistDraft(abs, id string, update GuideEntry) error {
	payload, err := encodeGuidePayload(update)
	if err != nil {
		return err
	}
	return s.store.UpsertDraft(abs, id, payload)
}

// editError sends the editor back to the edit form with the message rendered
// in the error banner. The handful of "save/preview/publish failed" branches
// all went through near-identical url.Values blocks; consolidating here keeps
// the error UX consistent and the handlers focused on their happy paths.
func (s *Server) editError(c *gin.Context, file, id, msg string) {
	s.redirectWithQuery(c, "/guides/edit", url.Values{
		"file":  {file},
		"id":    {id},
		"error": {msg},
	})
}

// draftKeySet returns the set of (file, id) keys that currently have drafts,
// for the list-view badge.
func (s *Server) draftKeySet() (map[string]bool, error) {
	drafts, err := s.store.ListDrafts()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(drafts))
	for _, d := range drafts {
		out[draftKey(d.FilePath, d.GuideID)] = true
	}
	return out, nil
}

// draftKey joins a (file, id) pair into a single string for the membership map.
// NUL is used as separator since paths can't contain it.
func draftKey(file, id string) string { return file + "\x00" + id }

// fileBytes is a thin wrapper around os.ReadFile so the diff path has a
// single chokepoint in case we add caching or layout-aware reads later.
func (s *Server) fileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// encodeGuidePayload serialises only the editable subset of a GuideEntry —
// FilePath / ID live in the table key columns so duplicating them in the JSON
// payload would just be data-drift waiting to happen.
func encodeGuidePayload(g GuideEntry) (string, error) {
	body := struct {
		Icon        string `json:"icon"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}{g.Icon, g.Title, g.Description}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeGuidePayload is the reverse — populates only the editable fields, the
// rest of the GuideEntry (FilePath, ID) is supplied by the caller.
func decodeGuidePayload(s string) (GuideEntry, error) {
	var body struct {
		Icon        string `json:"icon"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(s), &body); err != nil {
		return GuideEntry{}, err
	}
	return GuideEntry{Icon: body.Icon, Title: body.Title, Description: body.Description}, nil
}

// humanAgo renders a duration like "a few seconds ago" / "5 minutes ago" /
// "2 hours ago" — enough granularity to tell editors how stale their draft is
// without pulling in a time-formatting library.
func humanAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 45*time.Second:
		return "a few seconds ago"
	case d < 90*time.Second:
		return "a minute ago"
	case d < 45*time.Minute:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 90*time.Minute:
		return "an hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// ---- helpers ----

// resolveSpecPath turns a user-supplied (relative) path into an absolute one
// after asserting it lives inside specDir, with symlinks resolved on both
// sides. Rejects empty paths, ".." escapes, absolute paths pointing
// elsewhere, and symlinks that target outside specDir — the CMS must never
// write outside its configured root.
//
// EvalSymlinks on the candidate is the critical guard: filepath.Abs is a
// purely textual operation, so a symlink `<specDir>/escape -> /etc` would
// have an `abs` that lives inside specDir textually but resolves to /etc
// at IO time. We resolve and re-check the prefix on the canonical path.
//
// EvalSymlinks fails for non-existent files (the CMS only ever edits
// existing guides, so this is the expected case), so we resolve as far as
// the existing parent of the target and append the trailing components.
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
	resolved, err := evalSymlinksLenient(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	rootWithSep := s.specDir + string(filepath.Separator)
	if resolved != s.specDir && !strings.HasPrefix(resolved, rootWithSep) {
		return "", errors.New("path escapes spec dir")
	}
	return resolved, nil
}

// evalSymlinksLenient resolves symlinks for path, including for non-existent
// targets — it walks up to the deepest existing ancestor, resolves THAT, and
// re-joins the remaining tail. Plain filepath.EvalSymlinks errors out the
// moment any path component doesn't exist, which would reject any path the
// editor uses for a file not yet on disk (we don't do that today, but the
// guard should not become brittle if we add a "create new guide" flow).
func evalSymlinksLenient(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	parent, base := filepath.Split(path)
	parent = filepath.Clean(parent)
	if parent == path || parent == "" {
		return path, nil
	}
	resolvedParent, err := evalSymlinksLenient(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, base), nil
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
