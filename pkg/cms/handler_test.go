package cms

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestServer wires a CMS handler against an in-memory SQLite and a temp
// spec dir, then returns the gin engine + its store + a fresh logged-in
// session cookie so tests can hit authed routes directly.
func newTestServer(t *testing.T, password string) (*gin.Engine, *Store, string, *http.Cookie) {
	t.Helper()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	auth, err := NewAuthenticator(store, password)
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}

	dir := t.TempDir()
	srv, err := NewServer(store, auth, dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv.RegisterRoutes(r)

	sess, err := store.NewSession(AdminUser)
	if err != nil {
		t.Fatalf("mint session: %v", err)
	}
	cookie := &http.Cookie{Name: "cms_session", Value: sess.Token}
	return r, store, srv.specDir, cookie
}

// TestResolveSpecPath_RejectsSymlinkEscape is the regression guard for bug #1:
// filepath.Abs is purely textual, so a symlink inside specDir pointing
// outside the sandbox used to pass the HasPrefix(abs, specDir+sep) check.
// EvalSymlinks must follow the link before the prefix comparison.
func TestResolveSpecPath_RejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}
	specDir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.yaml")
	if err := os.WriteFile(target, []byte("guides: []\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(specDir, "escape.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	store, _ := OpenStore(":memory:")
	defer store.Close()
	auth, _ := NewAuthenticator(store, "p")
	srv, err := NewServer(store, auth, specDir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if _, err := srv.resolveSpecPath("escape.yaml"); err == nil {
		t.Fatal("expected symlink-escape to be rejected, got nil err")
	}
}

// TestResolveSpecPath_AllowsRealFilesInside is the partner test: a regular
// file inside specDir must still resolve cleanly after the EvalSymlinks
// hardening.
func TestResolveSpecPath_AllowsRealFilesInside(t *testing.T) {
	specDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "guides", "x.yaml"), []byte("guides: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	store, _ := OpenStore(":memory:")
	defer store.Close()
	auth, _ := NewAuthenticator(store, "p")
	srv, err := NewServer(store, auth, specDir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	got, err := srv.resolveSpecPath("guides/x.yaml")
	if err != nil {
		t.Fatalf("unexpected reject: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("guides", "x.yaml")) {
		t.Errorf("resolved path doesn't look right: %q", got)
	}
}

// TestPublish_RejectsEmptyTitle is the regression guard for bug #5: the
// publish handler used to write the YAML without re-validating Title. A
// draft hand-edited (or migrated) to have an empty Title would silently
// overwrite the disk title with "". Now we reject the publish.
func TestPublish_RejectsEmptyTitle(t *testing.T) {
	r, store, specDir, cookie := newTestServer(t, "p")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(specDir, "guides", "x.yaml")
	original := "guides:\n  - id: x\n    title: Original\n    description: orig\n"
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Insert a draft directly with empty title (simulating DB hand-edit /
	// migration / older binary) so we can verify the publish guard.
	if err := store.UpsertDraft(yamlPath, "x", `{"icon":"","title":"","description":"bad"}`); err != nil {
		t.Fatalf("insert draft: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"/guides/publish?file=guides/x.yaml&id=x", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/guides/edit") || !strings.Contains(loc, "error=") {
		t.Errorf("expected redirect to edit with error, got: %s", loc)
	}
	after, _ := os.ReadFile(yamlPath)
	if string(after) != original {
		t.Errorf("file was mutated despite empty-title guard:\n%s", after)
	}
}

// TestDraftDiscard_SuccessPath verifies the happy-path discard flow. The
// regression guard for bug #8 (DeleteDraft errors must surface, not show a
// false success flash) is covered by code review — simulating the failure
// from inside an httptest handler requires breaking the same store that
// serves the session lookup, which makes the test 401 before reaching the
// discard handler. The fix is the explicit error branch in
// handleGuideDraftDiscard.
func TestDraftDiscard_SuccessPath(t *testing.T) {
	r, store, specDir, cookie := newTestServer(t, "p")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(specDir, "guides", "x.yaml")
	if err := os.WriteFile(yamlPath, []byte("guides:\n  - id: x\n    title: T\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := store.UpsertDraft(yamlPath, "x", `{"icon":"","title":"T","description":"d"}`); err != nil {
		t.Fatalf("upsert draft: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"/guides/draft/discard?file=guides/x.yaml&id=x", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d (body=%s)", w.Code, w.Body.String())
	}
	if _, err := store.GetDraft(yamlPath, "x"); err != ErrDraftNotFound {
		t.Errorf("draft was not deleted, err=%v", err)
	}
}

// TestLegacyEditPost_RedirectsToPreview guards bug #14: the old POST
// /guides/edit (direct publish) is gone, so bookmarked URLs and form
// resubmits used to 404. We added a 307 shim that replays the body through
// the preview flow.
func TestLegacyEditPost_RedirectsToPreview(t *testing.T) {
	r, _, _, cookie := newTestServer(t, "p")
	req := httptest.NewRequest(http.MethodPost,
		"/guides/edit?file=guides/x.yaml&id=x",
		strings.NewReader("title=t"))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect, got %d (body=%s)", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/guides/preview") {
		t.Errorf("legacy POST should redirect to /guides/preview; got: %s", loc)
	}
	if !strings.Contains(loc, "file=guides/x.yaml") || !strings.Contains(loc, "id=x") {
		t.Errorf("redirect query was rewritten: %s", loc)
	}
}

// TestEditForm_URLEncodesFormActions guards bug #2: file paths and ids with
// `&` or other reserved chars used to break the formaction query because
// html/template only HTML-escaped them. The urlquery filter must produce
// URL-encoded values that round-trip.
func TestEditForm_URLEncodesFormActions(t *testing.T) {
	r, _, specDir, cookie := newTestServer(t, "p")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(specDir, "guides", "orders&refund.yaml")
	if err := os.WriteFile(yamlPath, []byte("guides:\n  - id: x\n    title: T\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/guides/edit?"+url.Values{"file": {"guides/orders&refund.yaml"}, "id": {"x"}}.Encode(),
		nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "orders%26refund.yaml") {
		t.Errorf("expected URL-encoded `orders%%26refund.yaml` in formaction; got actions:\n%s", extractFormActions(body))
	}
}

func extractFormActions(body string) string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "formaction=") || strings.Contains(line, "action=\"/guides/") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return strings.Join(lines, "\n")
}

// TestFlowEditor_EndToEnd verifies the full flow-editor path through the
// HTTP handler: form fields → collectFlowSteps → persistDraft → publish →
// YAML on disk. This is the integration guard that the Slice A feature
// hangs together — earlier unit tests verified each layer in isolation.
func TestFlowEditor_EndToEnd(t *testing.T) {
	r, store, specDir, cookie := newTestServer(t, "p")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(specDir, "guides", "upload.yaml")
	original := `guides:
  - id: upload
    title: T
    flow:
      - step: 1
        title: Step 1
        endpoint:
          method: POST
          path: /a
          service: Media Service
          permission: media:upload
      - step: 2
        title: Step 2
        endpoint: { method: GET, path: /b }
`
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Form: reorder to [2, 1], add new step. Use url.Values for clarity.
	body := url.Values{
		"title":         {"T"},
		"description":   {""},
		"icon":          {""},
		"step_count":    {"3"},
		"step_0_orig":   {"2"},
		"step_0_title":  {"Was step 2"},
		"step_0_method": {"GET"},
		"step_0_path":   {"/b"},
		"step_1_orig":   {"1"},
		"step_1_title":  {"Step 1 edited"},
		"step_1_method": {"PUT"}, // changed
		"step_1_path":   {"/a"},
		"step_2_orig":   {""},
		"step_2_title":  {"Brand new"},
		"step_2_method": {"DELETE"},
		"step_2_path":   {"/c"},
	}

	post := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost,
			path+"?file=guides/upload.yaml&id=upload",
			strings.NewReader(body.Encode()))
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := post("/guides/draft"); w.Code != http.StatusSeeOther {
		t.Fatalf("draft: status=%d body=%s", w.Code, w.Body.String())
	}
	// File is untouched after draft.
	after, _ := os.ReadFile(yamlPath)
	if string(after) != original {
		t.Fatalf("file mutated by draft save:\n%s", after)
	}
	// Draft has the new flow.
	d, err := store.GetDraft(yamlPath, "upload")
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	g, err := decodeGuidePayload(d.Payload)
	if err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if len(g.Flow) != 3 {
		t.Fatalf("want 3 flow steps in draft, got %d", len(g.Flow))
	}
	if g.Flow[0].OrigKey != "2" || g.Flow[1].OrigKey != "1" || g.Flow[2].OrigKey != "" {
		t.Errorf("draft flow OrigKeys wrong: [%q %q %q]",
			g.Flow[0].OrigKey, g.Flow[1].OrigKey, g.Flow[2].OrigKey)
	}

	// Publish.
	if w := post("/guides/publish"); w.Code != http.StatusSeeOther {
		t.Fatalf("publish: status=%d body=%s", w.Code, w.Body.String())
	}
	// Drain the form body to publish actually reads the latest from draft.
	// (Publish ignores the body — it reads from drafts table.)

	out, _ := os.ReadFile(yamlPath)
	s := string(out)
	// Order in file: step 2 first, then 1, then new.
	idx2 := strings.Index(s, "Was step 2")
	idx1 := strings.Index(s, "Step 1 edited")
	idxN := strings.Index(s, "Brand new")
	if idx2 < 0 || idx1 < 0 || idxN < 0 {
		t.Fatalf("not all flow step titles present in output:\n%s", s)
	}
	if !(idx2 < idx1 && idx1 < idxN) {
		t.Errorf("order wrong (want 2, 1, new; got positions %d %d %d):\n%s", idx2, idx1, idxN, s)
	}
	// Non-editable preserved on step 1.
	if !strings.Contains(s, "service: Media Service") {
		t.Errorf("endpoint.service was destroyed by flow edit:\n%s", s)
	}
	if !strings.Contains(s, "permission: media:upload") {
		t.Errorf("endpoint.permission was destroyed by flow edit:\n%s", s)
	}
	// Step 1 method updated.
	if !strings.Contains(s, "method: PUT") {
		t.Errorf("step 1 method should be PUT:\n%s", s)
	}
}

// TestFlowEditor_NoStepCountLeavesFlowAlone ensures the legacy metadata-only
// form (no step_count field) doesn't accidentally clear the flow.
func TestFlowEditor_NoStepCountLeavesFlowAlone(t *testing.T) {
	r, store, specDir, cookie := newTestServer(t, "p")
	if err := os.MkdirAll(filepath.Join(specDir, "guides"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(specDir, "guides", "x.yaml")
	original := "guides:\n  - id: x\n    title: T\n    flow:\n      - step: 1\n        title: keep me\n"
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	body := url.Values{
		"title":       {"T edited"},
		"description": {""},
		"icon":        {""},
		// no step_count — simulates a legacy / metadata-only post
	}
	req := httptest.NewRequest(http.MethodPost,
		"/guides/draft?file=guides/x.yaml&id=x",
		strings.NewReader(body.Encode()))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("draft: status=%d body=%s", w.Code, w.Body.String())
	}
	d, err := store.GetDraft(yamlPath, "x")
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	g, err := decodeGuidePayload(d.Payload)
	if err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if g.Flow != nil {
		t.Errorf("missing step_count should produce nil Flow (leave-existing semantics); got %+v", g.Flow)
	}
}
