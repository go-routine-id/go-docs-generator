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

// TestSaveGuide_FlowDuplicateOrigKey is the regression guard for the
// applyFlowEdits byKey-collision bug: two updates sharing an OrigKey used
// to both look up the SAME yaml.Node, last-write-wins, and emit two
// identical YAML rows.
func TestSaveGuide_FlowDuplicateOrigKey(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: A
        endpoint: { method: GET, path: /a, service: First }
      - step: 1
        title: B
        endpoint: { method: GET, path: /b, service: Second }
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "First", EndpointMethod: "GET", EndpointPath: "/a"},
			{OrigKey: "1", Title: "Second", EndpointMethod: "GET", EndpointPath: "/b"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	got, _ := LoadGuide(file, "x")
	if len(got.Flow) != 2 {
		t.Fatalf("want 2 steps after dup-key save, got %d", len(got.Flow))
	}
	if got.Flow[0].Title != "First" || got.Flow[1].Title != "Second" {
		t.Errorf("titles wrong: [%q %q]", got.Flow[0].Title, got.Flow[1].Title)
	}
}

// TestSaveGuide_EndpointNullReplacedNotMutated guards the endpoint:null
// silent-loss bug: applyFlowStep used to call setOrAppendScalar on a
// ScalarNode whose Content yaml.v3 ignores at marshal time, so the editor's
// method/path edits were silently discarded.
func TestSaveGuide_EndpointNullReplacedNotMutated(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: Placeholder
        endpoint: null
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "Placeholder", EndpointMethod: "GET", EndpointPath: "/a"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	if !strings.Contains(s, "method: GET") || !strings.Contains(s, "path: /a") {
		t.Errorf("method/path lost when source had `endpoint: null`:\n%s", s)
	}
	if strings.Contains(s, "endpoint: null") {
		t.Errorf("endpoint: null was not replaced:\n%s", s)
	}
}

// TestSaveGuide_NoEmptyCurlFields guards the diff-pollution fix: applyFlowStep
// used to write `curl_example_jwt:`, `curl_example_api_key:`, and
// `response_example:` even when the value was empty and the field didn't
// exist on disk, polluting every save with empty keys.
func TestSaveGuide_NoEmptyCurlFields(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: Old
        endpoint: { method: GET, path: /a }
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "New", EndpointMethod: "GET", EndpointPath: "/a"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	for _, polluted := range []string{
		"curl_example_jwt:",
		"curl_example_api_key:",
		"response_example:",
	} {
		if strings.Contains(s, polluted) {
			t.Errorf("empty optional field %q was emitted into a save that didn't set it\n%s", polluted, s)
		}
	}
}

// TestSaveGuide_FlowReorderDoesNotRewriteStepKey guards the dual-purpose-
// `step:` regression: applyFlowStep used to write OrigKey back as `step:`,
// producing non-contiguous step numbers (e.g. `step: 3` at array position
// 0) on every reorder. Now the existing `step:` field on each yaml.Node
// is preserved untouched — the CMS doesn't own that field.
func TestSaveGuide_FlowReorderDoesNotRewriteStepKey(t *testing.T) {
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
`)
	// Reorder to [2, 1] — the rows swap positions.
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "2", Title: "B", EndpointMethod: "GET", EndpointPath: "/b"},
			{OrigKey: "1", Title: "A", EndpointMethod: "GET", EndpointPath: "/a"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	// Step labels must travel with their original content — the row labelled
	// `step: 2` is now at position 0 (titled B), the row labelled `step: 1`
	// is now at position 1 (titled A). Crucially neither is mutated:
	// `step: 1` stays a bare integer, NOT promoted to `step: "1"`.
	if !strings.Contains(s, "step: 2") || !strings.Contains(s, "step: 1") {
		t.Errorf("step labels lost in reorder:\n%s", s)
	}
	if strings.Contains(s, "step: \"1\"") || strings.Contains(s, "step: '1'") {
		t.Errorf("integer step value was quote-promoted on save:\n%s", s)
	}
}

// TestSaveGuide_FlowStringTypeConfusion guards against scalar-style auto-
// typing: an editor's title or path that happens to look like "42", "true",
// "null" must round-trip as a STRING (single-quoted), not as int/bool/null.
func TestSaveGuide_FlowStringTypeConfusion(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: old
        endpoint: { method: GET, path: /a }
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "true", EndpointMethod: "GET", EndpointPath: "42"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if got.Flow[0].Title != "true" {
		t.Errorf("title \"true\" round-tripped as %q (likely promoted to bool)", got.Flow[0].Title)
	}
	if got.Flow[0].EndpointPath != "42" {
		t.Errorf("path \"42\" round-tripped as %q (likely promoted to int)", got.Flow[0].EndpointPath)
	}
}

// TestSaveGuide_FlowAssignsStepForNewSteps guards the "Step 0" rendering
// regression: previously, removing the OrigKey-as-step writeback meant
// newly-added steps had no `step:` field, and pkg/docs.Step (int) zero-
// defaulted them to "Step 0:" on the docs page. Now applyFlowStep assigns
// a positional integer when the node has no step: of its own.
func TestSaveGuide_FlowAssignsStepForNewSteps(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: existing
        endpoint: { method: GET, path: /a }
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "existing"},
			{OrigKey: "", Title: "NEW step", EndpointMethod: "POST", EndpointPath: "/b"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	// Existing step's author-assigned `step: 1` is preserved untouched.
	if !strings.Contains(s, "step: 1") {
		t.Errorf("existing step: 1 was lost:\n%s", s)
	}
	// New step got a synthesized step value of its position (2).
	if !strings.Contains(s, "step: 2") {
		t.Errorf("new step at position 2 should have synthesised step: 2; got:\n%s", s)
	}
	// Round-trip load: both flow steps have non-empty OrigKey now.
	got, _ := LoadGuide(file, "x")
	if len(got.Flow) != 2 {
		t.Fatalf("want 2 steps, got %d", len(got.Flow))
	}
	for i, st := range got.Flow {
		if st.OrigKey == "" {
			t.Errorf("step[%d] has no OrigKey — would render as Step 0 in docs", i)
		}
	}
}

// TestSaveGuide_FlowDuplicateKeyPreservesNestedFields is the regression
// guard for the symmetric dup-key bug: flowStepsFromSeq now disambiguates
// duplicate `step:` values with a #N suffix, and applyFlowEdits' byKey
// uses the same algorithm, so both rows' nested non-editable fields
// survive a no-op edit.
func TestSaveGuide_FlowDuplicateKeyPreservesNestedFields(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: First
        endpoint:
          method: GET
          path: /a
          service: First-Service
          permission: first:read
      - step: 1
        title: Second
        endpoint:
          method: POST
          path: /b
          service: Second-Service
          permission: second:write
`)
	// Read both back, then save with no edits (mirroring what a UI no-op
	// save would look like).
	got, err := LoadGuide(file, "x")
	if err != nil {
		t.Fatalf("LoadGuide: %v", err)
	}
	if len(got.Flow) != 2 {
		t.Fatalf("want 2 steps, got %d", len(got.Flow))
	}
	if got.Flow[0].OrigKey == got.Flow[1].OrigKey {
		t.Errorf("flowStepsFromSeq should disambiguate dup keys; both rows got OrigKey=%q", got.Flow[0].OrigKey)
	}
	if err := SaveGuide(file, GuideEntry{ID: "x", Title: "T", Flow: got.Flow}); err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	// BOTH services and BOTH permissions must survive — the headline
	// data-loss guard.
	for _, want := range []string{
		"service: First-Service",
		"service: Second-Service",
		"permission: first:read",
		"permission: second:write",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("dup-key save dropped %q from one of the rows:\n%s", want, s)
		}
	}
}

// TestSaveGuide_DeletesEmptyOptionalField guards setScalarOptional's
// clear-to-delete behavior: when an editor blanks a textarea whose field
// EXISTED on disk, the field is deleted from the YAML (not left as a
// stale `field: ”`).
func TestSaveGuide_DeletesEmptyOptionalField(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: T
        endpoint: { method: GET, path: /a }
        response_example: outdated
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "T", EndpointMethod: "GET", EndpointPath: "/a",
				ResponseExample: "" /* cleared */},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	if strings.Contains(s, "response_example:") {
		t.Errorf("response_example should be deleted when editor clears it; got:\n%s", s)
	}
	if strings.Contains(s, "outdated") {
		t.Errorf("outdated response_example value still in file:\n%s", s)
	}
}

// TestSaveGuide_HexOctalInfNanStayString extends the type-confusion guard
// to cover the YAML core schema cases that strconv didn't catch before:
// hex/octal/binary integers and the dotted-inf/nan float specials.
func TestSaveGuide_HexOctalInfNanStayString(t *testing.T) {
	cases := map[string]string{
		"hex":        "0x1F",
		"octal":      "0o17",
		"binary":     "0b101",
		"dot-inf":    ".inf",
		"dot-nan-mc": ".NaN",
		"plain-true": "true",
		"plain-42":   "42",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: orig
        endpoint: { method: GET, path: /a }
`)
			err := SaveGuide(file, GuideEntry{
				ID: "x", Title: "T",
				Flow: []FlowStep{
					{OrigKey: "1", Title: value, EndpointMethod: "GET", EndpointPath: value},
				},
			})
			if err != nil {
				t.Fatalf("SaveGuide: %v", err)
			}
			got, err := LoadGuide(file, "x")
			if err != nil {
				t.Fatalf("LoadGuide: %v", err)
			}
			if got.Flow[0].Title != value {
				t.Errorf("title %q round-tripped as %q — auto-typed by yaml.v3", value, got.Flow[0].Title)
			}
			if got.Flow[0].EndpointPath != value {
				t.Errorf("path %q round-tripped as %q — auto-typed by yaml.v3", value, got.Flow[0].EndpointPath)
			}
		})
	}
}

// TestSaveGuide_FlowInsertInMiddleDoesNotCollide is the regression guard for
// the step-value-collision bug. Inserting a new step in the middle of an
// existing [step:1, step:2, step:3] flow used to assign `step: 3` to the
// new row (position-based), producing two YAML entries with the same step
// label and TWO panels titled "Step 3:" on the docs page. Now applyFlowEdits
// assigns from max(existing-reused)+1, guaranteeing no collision.
func TestSaveGuide_FlowInsertInMiddleDoesNotCollide(t *testing.T) {
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
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "A", EndpointMethod: "GET", EndpointPath: "/a"},
			{OrigKey: "2", Title: "B", EndpointMethod: "GET", EndpointPath: "/b"},
			{OrigKey: "", Title: "NEW", EndpointMethod: "POST", EndpointPath: "/new"},
			{OrigKey: "3", Title: "C", EndpointMethod: "GET", EndpointPath: "/c"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)

	// Every original step value must still appear exactly once.
	for _, want := range []string{"step: 1", "step: 2", "step: 3"} {
		if strings.Count(s, want) != 1 {
			t.Errorf("%q appears %d times; insertion in middle must not collide:\n%s",
				want, strings.Count(s, want), s)
		}
	}
	// The new step must have a step: value > max existing (i.e. 4).
	if !strings.Contains(s, "step: 4") {
		t.Errorf("new step should be assigned step: 4 (max+1); got:\n%s", s)
	}
	// And it must be the one labelled NEW.
	got, _ := LoadGuide(file, "x")
	if len(got.Flow) != 4 {
		t.Fatalf("want 4 steps, got %d", len(got.Flow))
	}
	for _, st := range got.Flow {
		if st.Title == "NEW" && st.OrigKey != "4" {
			t.Errorf("NEW step should round-trip with OrigKey=\"4\"; got %q", st.OrigKey)
		}
	}
}

// TestSaveGuide_FlowMultipleNewStepsGetSequentialValues verifies that adding
// several new steps in one save produces sequential, collision-free step
// values starting at max+1 rather than re-using positions that might
// collide with existing values.
func TestSaveGuide_FlowMultipleNewStepsGetSequentialValues(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 5
        title: existing
        endpoint: { method: GET, path: /a }
`)
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "5", Title: "existing", EndpointMethod: "GET", EndpointPath: "/a"},
			{OrigKey: "", Title: "NEW_1", EndpointMethod: "GET", EndpointPath: "/1"},
			{OrigKey: "", Title: "NEW_2", EndpointMethod: "GET", EndpointPath: "/2"},
			{OrigKey: "", Title: "NEW_3", EndpointMethod: "GET", EndpointPath: "/3"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	// max existing was 5 → new steps should get 6, 7, 8 — NOT 2, 3, 4
	// (which would have been the position-based assignment).
	for _, want := range []string{"step: 5", "step: 6", "step: 7", "step: 8"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in output; max+1 assignment failed:\n%s", want, s)
		}
	}
	for _, mustNot := range []string{"step: 2", "step: 3", "step: 4"} {
		// These would only appear if we wrongly assigned by position.
		if strings.Contains(s, mustNot) {
			t.Errorf("found %q — assignment fell back to position-based and may collide:\n%s", mustNot, s)
		}
	}
}

// TestSaveGuide_FlowRemovedStepFreesNumber confirms that when an editor
// removes a step, its step value is freed for reuse by a new step in the
// same save (since maxReusedIntegerStep only considers nodes that are
// actually reused).
func TestSaveGuide_FlowRemovedStepFreesNumber(t *testing.T) {
	dir := t.TempDir()
	file := writeYAML(t, dir, "guides/x.yaml", `guides:
  - id: x
    title: T
    flow:
      - step: 1
        title: keep
        endpoint: { method: GET, path: /a }
      - step: 2
        title: REMOVED
        endpoint: { method: GET, path: /b }
`)
	// Editor removes "REMOVED" (OrigKey 2) and adds NEW.
	err := SaveGuide(file, GuideEntry{
		ID: "x", Title: "T",
		Flow: []FlowStep{
			{OrigKey: "1", Title: "keep", EndpointMethod: "GET", EndpointPath: "/a"},
			{OrigKey: "", Title: "NEW", EndpointMethod: "GET", EndpointPath: "/c"},
		},
	})
	if err != nil {
		t.Fatalf("SaveGuide: %v", err)
	}
	out, _ := os.ReadFile(file)
	s := string(out)
	// max-reused is just 1 (existing step:2 was REMOVED, doesn't count) →
	// NEW gets step: 2 (freed by removal).
	if !strings.Contains(s, "step: 2") {
		t.Errorf("removed step's number should be reclaimable; expected step: 2 on new step:\n%s", s)
	}
	if strings.Contains(s, "REMOVED") {
		t.Errorf("removed step still in YAML:\n%s", s)
	}
}
