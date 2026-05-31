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
	"strconv"
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
//
// As a one-shot upgrade migration, NewServer also rekeys any draft rows
// whose file_path was stored as a pre-EvalSymlinks textual abs path (from
// an older binary) so they remain reachable under the new resolved-key
// lookup. Pre-existing drafts on a deployment where specDir was a symlink
// would otherwise be silently orphaned.
func NewServer(store *Store, auth *Authenticator, specDir string) (*Server, error) {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		return nil, fmt.Errorf("resolve spec dir: %w", err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseFS(TemplateFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	srv := &Server{store: store, auth: auth, specDir: abs, tmpl: tmpl}
	if err := srv.migrateDraftPaths(); err != nil {
		slog.Warn("migrate draft paths (some drafts may stay orphaned)", "err", err)
	}
	return srv, nil
}

// migrateDraftPaths walks every draft row and, if its stored file_path
// resolves through symlinks to a path that still lives inside specDir,
// updates the row to use that resolved path. Idempotent — once a draft's
// path is canonical, EvalSymlinks returns the same value and the row is
// left alone. Drafts whose path no longer exists OR resolves outside
// specDir are LEFT in place (the operator can clean them up via the UI
// once they re-import the file, or directly via the DB).
func (s *Server) migrateDraftPaths() error {
	drafts, err := s.store.ListDrafts()
	if err != nil {
		return err
	}
	rootSep := s.specDir + string(filepath.Separator)
	for _, d := range drafts {
		resolved, err := filepath.EvalSymlinks(d.FilePath)
		if err != nil || resolved == d.FilePath {
			continue
		}
		if resolved != s.specDir && !strings.HasPrefix(resolved, rootSep) {
			slog.Warn("draft path resolves outside spec dir; leaving as-is",
				"old", d.FilePath, "resolved", resolved, "guide", d.GuideID)
			continue
		}
		if err := s.store.RekeyDraft(d.FilePath, d.GuideID, resolved); err != nil {
			slog.Warn("rekey draft path", "old", d.FilePath, "new", resolved, "guide", d.GuideID, "err", err)
			continue
		}
		slog.Info("migrated draft path", "old", d.FilePath, "new", resolved, "guide", d.GuideID)
	}
	return nil
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
// published YAML. If the on-disk guide is missing AND a draft exists, the
// form still renders (with draft values) so the editor has a discard path —
// otherwise a transient file-rename mid-flow would orphan the draft with
// no UI to clear it.
func (s *Server) handleGuideEditForm(c *gin.Context) {
	file, id := c.Query("file"), c.Query("id")
	abs, err := s.resolveSpecPath(file)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	// Try the disk first; tolerate ErrGuideNotFound and ENOENT-style errors
	// when we have a draft to render from (the editor can then Discard).
	guide, loadErr := LoadGuide(abs, id)
	guideOnDisk := loadErr == nil

	// Inspect draft state. The draft is shown for FOUR cases (healthy /
	// corrupt / DB-error / missing-disk-but-draft-exists) so the editor
	// always has a path to inspect or clear it.
	var (
		draftBanner  string
		hasDraft     bool
		draftCorrupt bool
	)
	d, dErr := s.store.GetDraft(abs, id)
	switch {
	case errors.Is(dErr, ErrDraftNotFound):
		// no draft — happy path
	case dErr != nil:
		hasDraft = true
		draftBanner = "Could not load draft: " + dErr.Error() + "."
	default:
		hasDraft = true
		drafted, decodeErr := decodeGuidePayload(d.Payload)
		if decodeErr != nil {
			draftCorrupt = true
			draftBanner = "A draft exists but its payload is corrupt (" + decodeErr.Error() + "). Discard it to start over."
		} else {
			if !guideOnDisk {
				// Synthesise a placeholder so the form has fields to render
				// — the editor's only useful action here is Discard.
				guide = &GuideEntry{ID: id}
			}
			guide.Icon = drafted.Icon
			guide.Title = drafted.Title
			guide.Description = drafted.Description
			draftBanner = "Editing an unsaved draft from " + humanAgo(d.UpdatedAt) + "."
		}
	}

	// No disk guide AND nothing useful in drafts — surface a 404 like before.
	if !guideOnDisk && guide == nil {
		if errors.Is(loadErr, ErrGuideNotFound) {
			c.String(http.StatusNotFound, "guide not found")
			return
		}
		slog.Error("load guide", "file", abs, "id", id, "err", loadErr)
		c.String(http.StatusInternalServerError, "load guide: "+loadErr.Error())
		return
	}

	// Corrupt-draft case: blank the form so Save Draft can't silently overwrite
	// the corrupt payload with disk values — force the editor to either type
	// new content explicitly or click Discard.
	if draftCorrupt {
		guide.Icon = ""
		guide.Title = ""
		guide.Description = ""
	}

	guide.FilePath = file
	s.render(c, "guide_edit", gin.H{
		"User":         sessionUser(c),
		"Guide":        guide,
		"DraftBanner":  draftBanner,
		"HasDraft":     hasDraft,
		"DraftCorrupt": draftCorrupt,
		"Flash":        c.Query("flash"),
		"Error":        c.Query("error"),
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
	s.editFlash(c, file, id, "Draft saved.")
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
		s.editError(c, file, id, "Discard failed: "+err.Error())
		return
	}
	s.editFlash(c, file, id, "Draft discarded.")
}

// handleGuidePreview persists the form as a draft FIRST so the editor's typed
// input is never lost to a transient render error, then reads the current
// YAML and renders the diff. If reading or rendering fails the editor is
// redirected back to /guides/edit with an explanation; their work is safe in
// the drafts table and the badge on the guides list reflects that.
//
// The edit form is tolerant of a missing-on-disk guide AS LONG AS a draft
// exists (handleGuideEditForm uses the draft as the source of truth when
// LoadGuide can't find anything), so the editor isn't stranded with a 404
// after a transient FS error.
func (s *Server) handleGuidePreview(c *gin.Context) {
	file, id, update, abs, ok := s.collectEditForm(c)
	if !ok {
		return
	}
	if err := s.persistDraft(abs, id, update); err != nil {
		slog.Error("preview: persist draft", "err", err)
		s.editError(c, file, id, "Save draft failed: "+err.Error())
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
		s.guidesFlash(c, "Nothing to publish — no draft for this guide.")
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
	s.guidesFlash(c, fmt.Sprintf("Published %s (%s).", id, file))
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
		Flow:        collectFlowSteps(c),
	}
	// Title was already TrimSpaced above; matches the publish-side guard
	// `strings.TrimSpace(update.Title) == ""` so whitespace-only titles
	// are rejected consistently at every gate.
	if strings.TrimSpace(update.Title) == "" {
		s.editError(c, file, id, "Title cannot be empty.")
		return file, id, update, "", false
	}
	return file, id, update, abs, true
}

// collectFlowSteps reads the per-step form fields the new flow editor
// submits. Returns nil — NOT an empty slice — when the form does not
// include a `step_count` hidden field at all, so legacy metadata-only
// posts and any caller that doesn't know about flow editing still get
// the "leave existing flow alone" semantics in SaveGuide.
//
// Form contract (i runs 0 .. step_count-1):
//
//	step_count                : integer count of submitted steps
//	step_<i>_orig             : hidden original key (e.g. "1", "2a", or "")
//	step_<i>_title            : step title
//	step_<i>_method           : endpoint.method
//	step_<i>_path             : endpoint.path
//	step_<i>_curl_jwt         : curl_example_jwt (multi-line)
//	step_<i>_curl_apikey      : curl_example_api_key (multi-line)
//	step_<i>_response         : response_example (multi-line)
//
// Steps appear in submission order; the editor uses up/down buttons to
// reorder them client-side before submit.
func collectFlowSteps(c *gin.Context) []FlowStep {
	raw := c.PostForm("step_count")
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return nil
	}
	out := make([]FlowStep, 0, n)
	for i := 0; i < n; i++ {
		p := "step_" + strconv.Itoa(i) + "_"
		title := strings.TrimSpace(c.PostForm(p + "title"))
		// Skip rows where the editor cleared the title — the editor probably
		// meant to delete the step but missed the Remove button. This is
		// conservative; if they really wanted an empty title, the
		// publish-time guard would reject the whole guide anyway.
		if title == "" {
			continue
		}
		out = append(out, FlowStep{
			OrigKey:         c.PostForm(p + "orig"),
			Title:           title,
			EndpointMethod:  strings.TrimSpace(c.PostForm(p + "method")),
			EndpointPath:    strings.TrimSpace(c.PostForm(p + "path")),
			CurlJWT:         c.PostForm(p + "curl_jwt"),
			CurlAPIKey:      c.PostForm(p + "curl_apikey"),
			ResponseExample: c.PostForm(p + "response"),
		})
	}
	return out
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

// editFlash is the sibling of editError for the success/info branches that
// land back on the edit form ("Draft saved.", "Draft discarded."). Keeps the
// flash-key contract in one place — adding a new success message no longer
// risks misspelling "flash" or forgetting the file/id round-trip.
func (s *Server) editFlash(c *gin.Context, file, id, msg string) {
	s.redirectWithQuery(c, "/guides/edit", url.Values{
		"file":  {file},
		"id":    {id},
		"flash": {msg},
	})
}

// guidesFlash redirects to the list view with a flash message (after
// publish / "nothing to publish"). Same rationale as editFlash.
func (s *Server) guidesFlash(c *gin.Context, msg string) {
	s.redirectWithQuery(c, "/guides", url.Values{"flash": {msg}})
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

// guidePayload is the serialised shape of a draft. Promoted to a named type
// so encode/decode can't drift apart (the prior anonymous-struct duplication
// was a real footgun: any field renamed in one and not the other would
// silently drop on round-trip). FilePath/ID are not part of the payload —
// they live in the table key columns.
//
// Flow is a pointer to a slice so nil-vs-empty is preserved across JSON:
//   - nil          → editor didn't touch the flow (metadata-only edit);
//     SaveGuide leaves the existing on-disk flow alone.
//   - non-nil []   → editor explicitly cleared every step;
//     SaveGuide writes an empty `flow:` sequence.
//   - non-nil [N]  → N steps in submission order.
type guidePayload struct {
	Icon        string      `json:"icon"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Flow        *[]FlowStep `json:"flow,omitempty"`
}

func encodeGuidePayload(g GuideEntry) (string, error) {
	body := guidePayload{
		Icon:        g.Icon,
		Title:       g.Title,
		Description: g.Description,
	}
	if g.Flow != nil {
		flow := g.Flow
		body.Flow = &flow
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeGuidePayload(s string) (GuideEntry, error) {
	var body guidePayload
	if err := json.Unmarshal([]byte(s), &body); err != nil {
		return GuideEntry{}, err
	}
	g := GuideEntry{Icon: body.Icon, Title: body.Title, Description: body.Description}
	if body.Flow != nil {
		g.Flow = *body.Flow
	}
	return g, nil
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
// We use STRICT filepath.EvalSymlinks (no lenient walk-up-for-missing-leaf
// fallback) so a DANGLING symlink — link exists, target doesn't — cannot
// pass through as if it were an ordinary path component and let the caller
// create the target outside specDir. The CMS only edits files that already
// exist on disk; a future create-new-guide flow MUST do its own resolution
// at the handler (EvalSymlinks the parent directory then join the leaf)
// rather than reach back through a shared lenient helper.
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
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	rootWithSep := s.specDir + string(filepath.Separator)
	if resolved != s.specDir && !strings.HasPrefix(resolved, rootWithSep) {
		return "", errors.New("path escapes spec dir")
	}
	return resolved, nil
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

// templateFuncs returns the small set of helpers the CMS templates need.
// `dict` is the standard escape hatch for passing multiple values into a
// sub-template (Go templates only accept one argument); `add` is the
// counterpart for the 1-based step index labels; `newFlowStep` provides
// the empty struct the JS template element renders for "Add step".
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"dict":        tmplDict,
		"add":         func(a, b int) int { return a + b },
		"newFlowStep": func() FlowStep { return FlowStep{} },
	}
}

// tmplDict packs alternating key/value arguments into a map so a sub-template
// can be invoked with multiple named parameters: {{template "x" (dict "A" 1 "B" 2)}}.
func tmplDict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, errors.New("dict expects an even number of arguments")
	}
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key at index %d is not a string", i)
		}
		out[key] = pairs[i+1]
	}
	return out, nil
}
