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
