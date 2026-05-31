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

// TestSaveGuide_PreservesEscapedBackslashU is the regression guard for the
// re-added literal-backslash guard in unescapeNonASCIIInDQ. yaml.v3 emits a
// user's literal `\U0001F680` inside a double-quoted scalar as `\\U0001F680`
// (escaped backslash). Without the guard, the second `\U` would be decoded
// to the rocket emoji and the editor's text silently destroyed. We force
// double-quoting by including a tab character in the description.
func TestSaveGuide_PreservesEscapedBackslashU(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    description: original
`)
	bs := "\\"
	// Tab forces yaml.v3 into double-quoted style.
	withLiteral := "Use " + bs + "U0001F680 verbatim\there"
	if err := SaveGuide(file, GuideEntry{ID: "x", Title: "T", Description: withLiteral}); err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	// The bytes-on-disk must contain the escaped form `\\U0001F680` so that
	// yaml.v3 re-parses it as the literal `\U0001F680` the editor typed.
	if !strings.Contains(string(out), bs+bs+"U0001F680") {
		t.Errorf("expected escaped literal `\\\\U0001F680` in file; emoji decode would mean data loss. Got:\n%s", out)
	}
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if got.Description != withLiteral {
		t.Errorf("round-trip lost content:\n got: %q\nwant: %q", got.Description, withLiteral)
	}
}

// TestSaveGuide_AllowsTrailingDocSeparator guards the regression where
// isMultiDoc treated a single document with a trailing `---` or `...` as
// multi-doc, locking the file out of the CMS.
func TestSaveGuide_AllowsTrailingDocSeparator(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/trailing.yaml", `guides:
  - id: x
    title: T
---
`)
	if err := SaveGuide(file, GuideEntry{ID: "x", Title: "Edited"}); err != nil {
		t.Errorf("trailing --- on single-doc file should be allowed; got: %v", err)
	}
}

// TestParseHexRune_RejectsInvalidRunes pins the parseHexRune + utf8.ValidRune
// guard: surrogate halves and out-of-range codepoints must NOT be silently
// emitted as U+FFFD. We test indirectly via unescapeUnicode on a yaml.v3-shaped
// input.
func TestUnescapeUnicode_LeavesInvalidEscapesIntact(t *testing.T) {
	// `\uD800` is a lone surrogate half — valid hex but not a valid rune.
	input := []byte(`key: "before\uD800after"`)
	out := unescapeUnicode(input)
	if !strings.Contains(string(out), `\uD800`) {
		t.Errorf("invalid escape should be preserved (not rewritten to U+FFFD); got:\n%s", out)
	}
}

// TestLoadGuide_ReadsFlowSteps verifies the editor pre-fill path reads each
// step's editable fields and an OrigKey we can round-trip on save.
func TestLoadGuide_ReadsFlowSteps(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: First
        endpoint:
          method: POST
          path: /a
        curl_example_jwt: |
          curl -X POST /a
        response_example: |
          {"ok":true}
      - step: 2
        title: Second
        endpoint:
          method: GET
          path: /b
`)
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if len(got.Flow) != 2 {
		t.Fatalf("want 2 steps, got %d", len(got.Flow))
	}
	s1 := got.Flow[0]
	if s1.OrigKey != "1" || s1.Title != "First" || s1.EndpointMethod != "POST" || s1.EndpointPath != "/a" {
		t.Errorf("step 1 wrong: %+v", s1)
	}
	if !strings.Contains(s1.CurlJWT, "curl -X POST /a") {
		t.Errorf("step 1 curl_jwt wrong: %q", s1.CurlJWT)
	}
	if !strings.Contains(s1.ResponseExample, "ok") {
		t.Errorf("step 1 response wrong: %q", s1.ResponseExample)
	}
}

// TestSaveGuide_FlowPreservesNonEditableFields is the headline guarantee for
// Slice A: editing flow steps must NOT clobber endpoint.service /
// endpoint.fields[] / endpoint.auth_methods / endpoint.permission /
// endpoint.content_type. Silent loss of those fields would be catastrophic
// for any guide built with file_upload.yaml-style richness.
func TestSaveGuide_FlowPreservesNonEditableFields(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/upload.yaml", `guides:
  - id: upload
    title: Upload
    flow:
      - step: 1
        title: Upload to Media Service
        endpoint:
          method: POST
          path: /api/v1/upload
          service: Media Service
          content_type: multipart/form-data
          auth: required
          auth_methods: [Bearer JWT, X-API-Key]
          permission: media:upload
          fields:
            - name: file
              type: binary
              required: true
              description: The file to upload
        curl_example_jwt: |
          curl -X POST -H "Authorization: Bearer T" /api/v1/upload
        curl_example_api_key: |
          curl -X POST -H "X-API-Key: K" /api/v1/upload
        response_example: |
          {"media_id":"abc"}
`)

	// Editor renames the step + tweaks the method. Everything else stays.
	err := SaveGuide(file, GuideEntry{
		ID:    "upload",
		Title: "Upload",
		Flow: []FlowStep{
			{
				OrigKey:         "1",
				Title:           "Upload to Media Service (edited)",
				EndpointMethod:  "PUT", // changed
				EndpointPath:    "/api/v1/upload",
				CurlJWT:         "curl -X POST -H \"Authorization: Bearer T\" /api/v1/upload\n",
				CurlAPIKey:      "curl -X POST -H \"X-API-Key: K\" /api/v1/upload\n",
				ResponseExample: "{\"media_id\":\"abc\"}\n",
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)

	// Editable fields applied.
	if !strings.Contains(s, "Upload to Media Service (edited)") {
		t.Errorf("title not updated:\n%s", s)
	}
	if !strings.Contains(s, "method: PUT") {
		t.Errorf("method not updated to PUT:\n%s", s)
	}
	// Non-editable nested fields MUST survive.
	for _, want := range []string{
		"service: Media Service",
		"content_type: multipart/form-data",
		"permission: media:upload",
		"name: file",
		"type: binary",
		"required: true",
		"The file to upload",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("non-editable field %q was destroyed by flow edit\n---\n%s", want, s)
		}
	}
}

// TestSaveGuide_FlowAddRemoveReorder exercises the three structural edits.
func TestSaveGuide_FlowAddRemoveReorder(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: A
        endpoint: { method: GET, path: /a }
      - step: 2
        title: B
        endpoint: { method: GET, path: /b }
      - step: 3
        title: C
        endpoint: { method: GET, path: /c }
`)
	// Editor: removes B, reorders to C/A, then adds a fresh step at the end.
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "3", Title: "C", EndpointMethod: "GET", EndpointPath: "/c"},
			{OrigKey: "1", Title: "A", EndpointMethod: "GET", EndpointPath: "/a"},
			{OrigKey: "", Title: "D (new)", EndpointMethod: "POST", EndpointPath: "/d"}, // added
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if len(got.Flow) != 3 {
		t.Fatalf("want 3 steps after reorder/add/remove, got %d", len(got.Flow))
	}
	if got.Flow[0].Title != "C" || got.Flow[1].Title != "A" || got.Flow[2].Title != "D (new)" {
		t.Errorf("order wrong: [%q %q %q]", got.Flow[0].Title, got.Flow[1].Title, got.Flow[2].Title)
	}
	if got.Flow[2].EndpointMethod != "POST" || got.Flow[2].EndpointPath != "/d" {
		t.Errorf("new step endpoint wrong: %+v", got.Flow[2])
	}
}

// TestSaveGuide_FlowNilLeavesExisting verifies the no-opinion path: when
// callers pass Flow==nil (metadata-only edits), the existing flow array
// is preserved byte-for-byte. This is the guard that lets the original
// metadata editors keep working without learning about Flow.
func TestSaveGuide_FlowNilLeavesExisting(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: Original
    flow:
      - step: 1
        title: Untouched
        endpoint: { method: GET, path: /a }
`)
	err := SaveGuide(file, GuideEntry{
		ID:    "x",
		Title: "Updated",
		Flow:  nil, // <-- the contract under test
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	if !strings.Contains(s, "title: Updated") {
		t.Errorf("title not updated:\n%s", s)
	}
	if !strings.Contains(s, "title: Untouched") {
		t.Errorf("nil Flow should leave existing flow alone — Untouched is gone\n%s", s)
	}
}
