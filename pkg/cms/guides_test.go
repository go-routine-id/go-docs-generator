package cms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeYAML drops content at path under dir, creating parent dirs as needed.
func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

func TestDiscoverGuides_FindsMultipleAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "guides/file_upload.yaml", `guides:
  - id: file_upload
    icon: "📤"
    title: File Upload Flow
    description: Upload via Media Service
`)
	writeYAML(t, dir, "guides/checkout.yaml", `guides:
  - id: cart_to_payment
    icon: 🛒
    title: Cart → Order → Payment
    description: Checkout journey
  - id: refund
    icon: 💸
    title: Refund flow
    description: Async refund
`)
	// Non-guides YAML (should be ignored).
	writeYAML(t, dir, "sections/users.yaml", `sections:
  - id: users
    title: Users
`)

	got, err := DiscoverGuides(dir)
	if err != nil {
		t.Fatalf("DiscoverGuides: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 guides, got %d (%+v)", len(got), got)
	}

	// IDs must include all three; ordering must be stable (file path then id).
	wantIDs := []string{"cart_to_payment", "refund", "file_upload"}
	for i, w := range wantIDs {
		if got[i].ID != w {
			t.Errorf("guide[%d].ID = %q, want %q", i, got[i].ID, w)
		}
	}
}

// TestSaveGuide_PreservesFlowAndComments is the headline guarantee: editing
// title/icon/description must NOT clobber the rich `flow:` array, nor any
// surrounding comments. Silent data loss here would be catastrophic — a guide
// has dozens of carefully-curated steps a CMS edit must keep intact.
func TestSaveGuide_PreservesFlowAndComments(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/file_upload.yaml", `# File Upload Guide
guides:
  - id: file_upload
    icon: "📤"
    title: File Upload Flow
    description: Original description
    flow:
      - step: 1
        title: Upload to Media Service
        endpoint:
          method: POST
          path: /api/v1/upload
          auth: required
        curl_example_jwt: |
          curl -X POST https://example.com/upload
      - step: 2
        title: Save media_id
`)

	err := SaveGuide(file, GuideEntry{
		ID:          "file_upload",
		Icon:        "🚀",
		Title:       "Updated Title",
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}

	out, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	s := string(out)

	// Editable fields updated.
	for _, want := range []string{"Updated Title", "Updated description", "🚀"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing edited value %q\n---\n%s", want, s)
		}
	}
	// Non-editable rich fields preserved verbatim — this is the
	// data-loss guard.
	for _, want := range []string{
		"step: 1",
		"step: 2",
		"endpoint:",
		"method: POST",
		"path: /api/v1/upload",
		"curl_example_jwt:",
		"Save media_id",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output lost non-editable content %q — would silently destroy author work\n---\n%s", want, s)
		}
	}
	// Old values must be gone (we're updating, not appending).
	for _, gone := range []string{"File Upload Flow", "Original description"} {
		if strings.Contains(s, gone) {
			t.Errorf("output still contains stale value %q after update\n---\n%s", gone, s)
		}
	}
}

func TestSaveGuide_UnknownIDReturnsErrGuideNotFound(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: real_guide
    title: Real
`)
	err := SaveGuide(file, GuideEntry{ID: "ghost"})
	if err != ErrGuideNotFound {
		t.Errorf("err = %v, want ErrGuideNotFound", err)
	}
}

// TestSaveGuide_OtherGuidesInFileUntouched protects against a regression
// where editing one guide in a multi-guide file accidentally rewrites the
// siblings.
func TestSaveGuide_OtherGuidesInFileUntouched(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/multi.yaml", `guides:
  - id: alpha
    title: Alpha
    description: First
    flow:
      - step: 1
        title: alpha-step
  - id: beta
    title: Beta
    description: Second
    flow:
      - step: 1
        title: beta-step
`)
	if err := SaveGuide(file, GuideEntry{
		ID:    "alpha",
		Title: "Alpha (edited)",
	}); err != nil {
		t.Fatalf("SaveGuide alpha: %v", err)
	}

	out, _ := os.ReadFile(file)
	s := string(out)

	if !strings.Contains(s, "Alpha (edited)") {
		t.Errorf("alpha title was not updated:\n%s", s)
	}
	for _, want := range []string{
		"id: beta",
		"title: Beta",
		"Second",
		"beta-step",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("editing alpha clobbered beta content %q\n---\n%s", want, s)
		}
	}
}

// TestProposedGuideYAML_DoesNotWriteFile is the key safety guarantee for the
// preview path: calling it must NOT mutate the file on disk — only return what
// SaveGuide WOULD have written. The preview page shows the diff, then the
// editor decides whether to commit. A leak here would mean unintentional
// publishes during preview.
func TestProposedGuideYAML_DoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    icon: ⭐
    title: Star
    description: Shiny
`)
	before, _ := os.ReadFile(file)

	out, err := ProposedGuideYAML(file, GuideEntry{
		ID: "x", Icon: "✨", Title: "Sparkle", Description: "Brighter",
	})
	if err != nil {
		t.Fatalf("ProposedGuideYAML: %v", err)
	}
	after, _ := os.ReadFile(file)

	if string(before) != string(after) {
		t.Errorf("file was mutated during preview render — this would publish unintentionally\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if !strings.Contains(string(out), "Sparkle") {
		t.Errorf("proposed bytes should contain the new title, got:\n%s", out)
	}
	if !strings.Contains(string(out), "Brighter") {
		t.Errorf("proposed bytes should contain the new description, got:\n%s", out)
	}
}

// TestSaveGuide_PreservesLiteralBackslashU is the regression guard for bug #3:
// unescapeUnicode used to rewrite ANY backslash-u-XXXX byte pattern in the
// marshaled output, including ones the editor typed literally. An editor's
// description containing the six-char literal sequence backslash+u+0+0+4+1
// must survive round-trip unchanged.
func TestSaveGuide_PreservesLiteralBackslashU(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    description: original
`)
	bs := "\\"
	literal := "Use " + bs + "u0041 to escape A and " + bs + "U0001F680 for the rocket."
	if err := SaveGuide(file, GuideEntry{ID: "x", Title: "T", Description: literal}); err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	if !strings.Contains(string(out), bs+"u0041") {
		t.Errorf("literal backslash-u0041 was destroyed by unescapeUnicode — output:\n%s", out)
	}
	if !strings.Contains(string(out), bs+"U0001F680") {
		t.Errorf("literal backslash-U0001F680 was destroyed by unescapeUnicode — output:\n%s", out)
	}
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide after save: %v", err)
	}
	if got.Description != literal {
		t.Errorf("round-trip lost content:\n got: %q\nwant: %q", got.Description, literal)
	}
}

// TestSaveGuide_PreservesEmojiUnescaped guards the OTHER direction of the same
// bug: emoji icons MUST be unescaped (otherwise the file is unreadable for
// humans), even though we now leave ASCII escapes alone.
func TestSaveGuide_PreservesEmojiUnescaped(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    icon: ⭐
    title: T
`)
	if err := SaveGuide(file, GuideEntry{ID: "x", Icon: "🚀", Title: "T"}); err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	if strings.Contains(string(out), `\U0001F680`) {
		t.Errorf("emoji icon was left escaped instead of unescaped — output:\n%s", out)
	}
	if !strings.Contains(string(out), "🚀") {
		t.Errorf("emoji icon missing from output:\n%s", out)
	}
}

// TestSaveGuide_PreservesControlCharEscapes is the regression guard for bug #4:
// unescapeUnicode used to decode control-character escapes into raw control
// bytes that yaml.v3 cannot re-parse from a plain scalar. The escapes must
// stay as escapes. yaml.v3 emits ESC (0x1B) as the short form "\e", so we
// verify that survives unchanged and round-trips back to ESC on parse.
func TestSaveGuide_PreservesControlCharEscapes(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    description: original
`)
	bs := "\\"
	escEmit := bs + "e" // yaml.v3 emits ESC as \e
	withEsc := "an ESC \x1b in the middle"
	if err := SaveGuide(file, GuideEntry{ID: "x", Title: "T", Description: withEsc}); err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	if strings.Contains(string(out), "\x1b") {
		t.Errorf("control char (0x1B) was written as a raw byte:\n%q", out)
	}
	if !strings.Contains(string(out), escEmit) {
		t.Errorf("yaml.v3's \\e escape form should be preserved; got:\n%s", out)
	}
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide after save: %v", err)
	}
	if got.Description != withEsc {
		t.Errorf("control-char round-trip wrong:\n got: %q\nwant: %q", got.Description, withEsc)
	}
}

// TestSaveGuide_RejectsMultiDocYAML is the regression guard for bug #9:
// yaml.v3 silently ignores everything past Content[0]; SaveGuide used to be
// able to edit only the first document while the editor thought they were
// targeting a guide in the second. Refuse the file with a clear error.
func TestSaveGuide_RejectsMultiDocYAML(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/multi.yaml", `guides:
  - id: alpha
    title: Alpha
---
guides:
  - id: beta
    title: Beta
`)
	err := SaveGuide(file, GuideEntry{ID: "alpha", Title: "edited"})
	if err == nil {
		t.Fatal("SaveGuide should reject multi-document YAML, got nil err")
	}
	if !strings.Contains(err.Error(), "multi-document") {
		t.Errorf("error should mention multi-document; got: %v", err)
	}
}

// TestDiscoverGuides_RejectsMultiDocYAML guards the read side: a multi-doc
// file must fail discovery so the editor sees the error in the list view
// instead of silently editing only doc 1.
func TestDiscoverGuides_RejectsMultiDocYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "guides/multi.yaml", `guides:
  - id: alpha
    title: Alpha
---
guides:
  - id: beta
    title: Beta
`)
	_, err := DiscoverGuides(dir)
	if err == nil || !strings.Contains(err.Error(), "multi-document") {
		t.Errorf("DiscoverGuides should reject multi-doc files; got err=%v", err)
	}
}

// TestLoadGuide_ReadsFields verifies the pre-fill path the edit form depends on.
func TestLoadGuide_ReadsFields(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    icon: ⭐
    title: Star
    description: Shiny
`)
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if got.Title != "Star" || got.Icon != "⭐" || got.Description != "Shiny" {
		t.Errorf("got %+v", got)
	}
}
