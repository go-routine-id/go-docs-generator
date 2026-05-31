package cms

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// GuideEntry is the flat view of one guide as the CMS shows it to editors.
// FilePath + ID together identify where the guide lives on disk so we know
// where to write back on publish.
type GuideEntry struct {
	FilePath    string
	ID          string
	Icon        string
	Title       string
	Description string
	Flow        []FlowStep
}

// FlowStep mirrors the editable subset of a single step inside a guide's
// `flow:` array. Non-editable nested fields (endpoint.service, endpoint.fields[],
// endpoint.auth_methods, endpoint.permission, endpoint.content_type, and any
// future addition) are NOT mirrored here — the round-trip preserves them
// byte-for-byte by walking yaml.Node directly.
//
// OrigKey is opaque to the editor; the form ships it back as a hidden field
// so the server can match a submitted step to its original yaml.Node and
// keep the non-editable nested fields intact. New steps (added via the UI)
// carry OrigKey == "" so the server knows to synthesise a fresh node.
type FlowStep struct {
	OrigKey         string // opaque key for round-trip matching; "" = newly added
	Title           string
	EndpointMethod  string
	EndpointPath    string
	CurlJWT         string
	CurlAPIKey      string
	ResponseExample string
}

// editableField lists the keys the CMS lets editors mutate. Anything outside
// this list is preserved byte-for-byte on round-trip (including the rich
// `flow:` array, comments, and key ordering).
var editableFields = []string{"icon", "title", "description"}

// DiscoverGuides scans dir recursively for YAML files containing a top-level
// `guides:` sequence and returns one GuideEntry per guide. The order is stable
// (file path, then guide index) so the CMS list view doesn't shuffle on reload.
func DiscoverGuides(dir string) ([]GuideEntry, error) {
	var out []GuideEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			return nil
		}
		entries, err := guidesInFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, entries...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// guidesInFile parses one YAML file and returns the guides declared in it. Files
// without a top-level `guides:` key contribute nothing (they may still be valid
// docs YAML — sections, info, etc. — but they're not edit surfaces here).
func guidesInFile(path string) ([]GuideEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, seq, err := parseGuidesDoc(path, raw)
	if err != nil {
		return nil, err
	}
	if doc == nil || seq == nil {
		return nil, nil
	}
	var out []GuideEntry
	for _, g := range seq.Content {
		if g.Kind != yaml.MappingNode {
			continue
		}
		entry := GuideEntry{FilePath: path}
		if n := childValue(g, "id"); n != nil {
			entry.ID = n.Value
		}
		if n := childValue(g, "icon"); n != nil {
			entry.Icon = n.Value
		}
		if n := childValue(g, "title"); n != nil {
			entry.Title = n.Value
		}
		if n := childValue(g, "description"); n != nil {
			entry.Description = n.Value
		}
		if entry.ID == "" {
			continue // ID-less guide isn't editable — we can't address it later
		}
		if flowSeq := childValue(g, "flow"); flowSeq != nil && flowSeq.Kind == yaml.SequenceNode {
			entry.Flow = flowStepsFromSeq(flowSeq)
		}
		out = append(out, entry)
	}
	return out, nil
}

// applyFlowEdits replaces the guide's `flow:` sequence with one assembled
// from `updates`, while preserving the nested non-editable fields of any
// existing step the editor matched by OrigKey. The rules:
//
//   - updates == nil → caller has no opinion about flow; leave the existing
//     sequence untouched. This is the metadata-only edit path (callers
//     that only set icon/title/description).
//   - update.OrigKey != "" → match an existing step by key; reuse its
//     yaml.Node so endpoint.service / endpoint.fields[] / auth_methods /
//     permission / content_type and any future field stay byte-identical.
//   - update.OrigKey == "" → newly added step; synthesise a minimal mapping
//     with only the editable fields.
//   - Existing steps NOT referenced by any update → dropped (the editor
//     used the remove button).
//   - Output order follows `updates` so reordering works naturally.
//
// If the guide has no `flow:` field but updates is non-empty, a new
// sequence is appended. Editors who explicitly remove every step (sending
// a non-nil zero-length slice) end up with an empty `flow:` sequence
// rather than the field being deleted entirely; that keeps the YAML
// shape stable for downstream tooling that expects the key to exist.
func applyFlowEdits(guide *yaml.Node, updates []FlowStep) {
	if updates == nil {
		return
	}
	existing := childValue(guide, "flow")
	byKey := map[string]*yaml.Node{}
	if existing != nil && existing.Kind == yaml.SequenceNode {
		for i, n := range existing.Content {
			byKey[flowStepKey(n, i)] = n
		}
	}

	newContent := make([]*yaml.Node, 0, len(updates))
	for _, u := range updates {
		var node *yaml.Node
		if u.OrigKey != "" {
			node = byKey[u.OrigKey]
		}
		if node == nil {
			node = &yaml.Node{Kind: yaml.MappingNode}
		}
		applyFlowStep(node, u)
		newContent = append(newContent, node)
	}

	if existing != nil {
		existing.Content = newContent
		return
	}
	// No prior flow field — append one.
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "flow"}
	seqNode := &yaml.Node{Kind: yaml.SequenceNode, Content: newContent}
	guide.Content = append(guide.Content, keyNode, seqNode)
}

// applyFlowStep mutates one step mapping in place to match the editable
// fields of u. `step:`, `title`, `curl_example_jwt`, `curl_example_api_key`,
// and `response_example` live at the top of the step; method/path live one
// level deeper inside `endpoint:`. All other fields on the node — service,
// auth_methods, permission, content_type, fields[] — are left alone.
func applyFlowStep(node *yaml.Node, u FlowStep) {
	setOrAppendScalar(node, "title", u.Title)
	// Preserve OrigKey as the step's `step:` value when it was a real
	// author-declared key (e.g. "1", "2a"); skip the synthetic "idx:N"
	// placeholder so we don't leak that into authored YAML.
	if u.OrigKey != "" && !strings.HasPrefix(u.OrigKey, "idx:") {
		setOrAppendScalar(node, "step", u.OrigKey)
	}

	ep := childValue(node, "endpoint")
	if ep == nil {
		// Endpoint mapping doesn't exist yet — create one with just the
		// editable fields. Non-editable sub-fields will be added by the
		// editor through some future flow (or by manual YAML editing).
		ep = &yaml.Node{Kind: yaml.MappingNode}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "endpoint"},
			ep,
		)
	}
	setOrAppendScalar(ep, "method", u.EndpointMethod)
	setOrAppendScalar(ep, "path", u.EndpointPath)

	setOrAppendScalar(node, "curl_example_jwt", u.CurlJWT)
	setOrAppendScalar(node, "curl_example_api_key", u.CurlAPIKey)
	setOrAppendScalar(node, "response_example", u.ResponseExample)
}

// flowStepsFromSeq reads a `flow:` sequence into the editor's flat view.
// OrigKey is the textual `step:` value (e.g. "1", "2a") because that's how
// authors anchor their flows in prose; if absent we fall back to the index
// so the position is still round-trippable. The form ships OrigKey back so
// SaveGuide can match a submitted step to its source yaml.Node and preserve
// nested non-editable fields verbatim.
func flowStepsFromSeq(seq *yaml.Node) []FlowStep {
	out := make([]FlowStep, 0, len(seq.Content))
	for i, n := range seq.Content {
		if n.Kind != yaml.MappingNode {
			continue
		}
		step := FlowStep{OrigKey: flowStepKey(n, i)}
		if v := childValue(n, "title"); v != nil {
			step.Title = v.Value
		}
		if ep := childValue(n, "endpoint"); ep != nil && ep.Kind == yaml.MappingNode {
			if v := childValue(ep, "method"); v != nil {
				step.EndpointMethod = v.Value
			}
			if v := childValue(ep, "path"); v != nil {
				step.EndpointPath = v.Value
			}
		}
		if v := childValue(n, "curl_example_jwt"); v != nil {
			step.CurlJWT = v.Value
		}
		if v := childValue(n, "curl_example_api_key"); v != nil {
			step.CurlAPIKey = v.Value
		}
		if v := childValue(n, "response_example"); v != nil {
			step.ResponseExample = v.Value
		}
		out = append(out, step)
	}
	return out
}

// flowStepKey returns a stable identifier for a step node, preferring the
// author-declared `step:` value and falling back to the array index. Used
// as the form's hidden round-trip key — never shown to the editor.
func flowStepKey(n *yaml.Node, idx int) string {
	if v := childValue(n, "step"); v != nil && v.Value != "" {
		return v.Value
	}
	return fmt.Sprintf("idx:%d", idx)
}

// ErrGuideNotFound is returned when SaveGuide / LoadGuide can't find a guide
// with the requested id in the file.
var ErrGuideNotFound = errors.New("guide not found in file")

// LoadGuide reads one guide from a file by id. Used by the edit form to
// pre-fill the inputs.
func LoadGuide(path, id string) (*GuideEntry, error) {
	entries, err := guidesInFile(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, ErrGuideNotFound
}

// SaveGuide writes the proposed YAML to disk atomically. Splits cleanly from
// ProposedGuideYAML so the same byte stream can be rendered for preview/diff
// without committing it.
func SaveGuide(path string, update GuideEntry) error {
	out, err := ProposedGuideYAML(path, update)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, out, 0o644)
}

// ProposedGuideYAML returns the bytes that SaveGuide would write — without
// actually writing them. The preview/diff path uses this to show the editor
// exactly what publishing will do before they commit. Reads the file once;
// for paths that already have the current bytes in hand, use
// ProposedGuideYAMLFromBytes to skip the extra read.
func ProposedGuideYAML(path string, update GuideEntry) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ProposedGuideYAMLFromBytes(path, raw, update)
}

// ProposedGuideYAMLFromBytes mutates the parsed view of `raw` to reflect the
// editable fields in update and returns the marshaled bytes. path is used
// only for error context. Pulled out so callers (the preview handler) can
// produce both the unchanged baseline and the proposed result from one read.
func ProposedGuideYAMLFromBytes(path string, raw []byte, update GuideEntry) ([]byte, error) {
	doc, seq, err := parseGuidesDoc(path, raw)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("%s: top-level YAML is not a mapping", path)
	}
	if seq == nil {
		return nil, fmt.Errorf("%s: no top-level `guides:` sequence", path)
	}

	target := findGuideByID(seq, update.ID)
	if target == nil {
		return nil, ErrGuideNotFound
	}

	setOrAppendScalar(target, "icon", update.Icon)
	setOrAppendScalar(target, "title", update.Title)
	setOrAppendScalar(target, "description", update.Description)
	applyFlowEdits(target, update.Flow)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}
	// yaml.v3 always emits non-ASCII as \U / \u escapes regardless of the
	// requested scalar style; convert ONLY the non-ASCII (>= 0x80) ones
	// back to UTF-8 so icons like "📤" stay readable. ASCII escapes
	// ( - ) are left intact: those are either (a) control
	// chars that MUST stay escaped to keep the YAML valid, or (b) a
	// literal `A` the editor typed in their description, which we
	// would silently rewrite into `A` if we touched ASCII codepoints.
	return unescapeUnicode(out), nil
}

// isMultiDoc returns true when raw contains more than one YAML document with
// non-empty content. yaml.v3 silently ignores everything past the first
// DocumentNode, so DiscoverGuides and ProposedGuideYAML would be invisible
// to (and would corrupt edits of) guides living past a `---` separator.
//
// Empty/null documents — emitted by yaml.v3 for trailing `---`/`...` markers
// or for `---\n---\n` runs — are NOT counted, otherwise a legitimate single-
// document file with a trailing end-marker would be falsely rejected.
func isMultiDoc(raw []byte) bool {
	d := yaml.NewDecoder(bytes.NewReader(raw))
	count := 0
	for {
		var n yaml.Node
		if err := d.Decode(&n); err != nil {
			break
		}
		if isEmptyDocNode(&n) {
			continue
		}
		count++
		if count > 1 {
			return true
		}
	}
	return false
}

// isEmptyDocNode reports whether a top-level decoded node carries no actual
// content — the shapes yaml.v3 emits for `---\n` separators with nothing
// (or only null) between them.
func isEmptyDocNode(n *yaml.Node) bool {
	if n == nil || n.Kind == 0 {
		return true
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) == 0 {
		return true
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		inner := n.Content[0]
		if inner == nil || inner.Kind == 0 {
			return true
		}
		if inner.Kind == yaml.ScalarNode && (inner.Tag == "!!null" || inner.Value == "") {
			return true
		}
	}
	return false
}

// unescapeUnicode rewrites \uXXXX / \UXXXXXXXX escape sequences inside
// yaml.v3-emitted double-quoted scalars back into UTF-8, so that icons like
// "📤" stay readable rather than rendering as "\U0001F4E4". Scoping the
// rewrite to "..." regions is critical: yaml.v3 emits user content in plain
// style verbatim (the YAML spec only interprets \u escapes in double-quoted
// scalars), so a literal "A" typed by the editor must survive
// untouched. We also restrict to codepoints >= 0x80 — inside a double-quoted
// string, an ASCII \uXXXX is much more likely a literal the editor wrote
// than a yaml.v3 emit, and control chars MUST stay escaped for the YAML to
// remain valid.
//
// The line-based scan handles the typical yaml.v3 output (one scalar per
// line) and gracefully leaves anything it can't pair (unterminated quotes)
// alone.
func unescapeUnicode(b []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(b))
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		out.Write(unescapeDoubleQuotedRegions(line))
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// unescapeDoubleQuotedRegions walks one YAML line, finds each well-formed
// "..." segment (handling \" and \\ escape pairs inside), and unescapes
// only the non-ASCII Unicode escapes within those segments. Bytes outside
// any quoted region are passed through untouched.
func unescapeDoubleQuotedRegions(line []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(line))
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '"' {
			end := findQuoteClose(line, i+1)
			if end >= 0 {
				out.WriteByte('"')
				out.Write(unescapeNonASCIIInDQ(line[i+1 : end]))
				out.WriteByte('"')
				i = end + 1
				continue
			}
			// Unterminated — give up scanning this line and copy verbatim
			// so we never corrupt content we can't structure.
			out.Write(line[i:])
			return out.Bytes()
		}
		out.WriteByte(c)
		i++
	}
	return out.Bytes()
}

// findQuoteClose returns the index of the matching unescaped close-quote
// starting from start, or -1 if the line ends without one.
func findQuoteClose(line []byte, start int) int {
	for j := start; j < len(line); j++ {
		if line[j] == '\\' && j+1 < len(line) {
			j++ // skip the escaped char
			continue
		}
		if line[j] == '"' {
			return j
		}
	}
	return -1
}

// unescapeNonASCIIInDQ decodes \uXXXX / \UXXXXXXXX → UTF-8 inside the
// already-validated double-quoted span, only for codepoints >= 0x80.
//
// The literal-backslash guard (skip when the preceding byte is itself `\`)
// is critical: yaml.v3 emits a user's literal `\U0001F680` as `\\U0001F680`
// (two backslashes). Without the guard, the second `\U` would be decoded
// as a real unicode escape and the editor's text would be silently
// destroyed. parseHexRune-derived runes are also validated against
// utf8.ValidRune so surrogate halves and out-of-range codepoints don't
// silently turn into U+FFFD.
func unescapeNonASCIIInDQ(span []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(span))
	for i := 0; i < len(span); i++ {
		c := span[i]
		// Only treat \u / \U as escape introducers when the preceding byte
		// isn't itself a backslash — otherwise this is the second half of
		// an escaped literal backslash that opens user content.
		if c == '\\' && i+1 < len(span) && (i == 0 || span[i-1] != '\\') {
			nx := span[i+1]
			width := 0
			switch nx {
			case 'u':
				width = 4
			case 'U':
				width = 8
			}
			if width > 0 && i+2+width <= len(span) {
				if r, ok := parseHexRune(span[i+2 : i+2+width]); ok && r >= 0x80 && utf8.ValidRune(r) {
					out.WriteString(string(r))
					i += 1 + width
					continue
				}
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}

// parseHexRune turns a hex byte slice into a rune. Returns false on any
// non-hex byte so the caller can leave the escape unchanged.
func parseHexRune(b []byte) (rune, bool) {
	var r rune
	for _, c := range b {
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case c >= 'a' && c <= 'f':
			d = rune(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			d = rune(c - 'A' + 10)
		default:
			return 0, false
		}
		r = r<<4 | d
	}
	return r, true
}

// parseGuidesDoc runs the shared prologue: reject multi-doc → yaml.Unmarshal
// → strip the DocumentNode wrapper → return the `guides:` sequence if present.
//
// The caller distinguishes three success states by the (doc, seq) tuple:
//   - (nil, nil)            top-level YAML is not a mapping (caller may want
//     to treat as "no guides here" or as an error)
//   - (doc, nil)            it IS a mapping but has no top-level `guides:`
//   - (doc, seq)            we have a `guides:` sequence to operate on
//
// Centralising this prologue keeps the multi-doc rejection message, the
// "parse yaml" error wrap, and the field-name "guides" all in one place
// so future schema edits land once instead of three times.
func parseGuidesDoc(path string, raw []byte) (root *yaml.Node, guides *yaml.Node, err error) {
	if isMultiDoc(raw) {
		return nil, nil, fmt.Errorf("%s: multi-document YAML files are not supported by the CMS (split into one document per file)", path)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse yaml: %w", err)
	}
	r := documentRoot(&doc)
	if r == nil || r.Kind != yaml.MappingNode {
		return nil, nil, nil
	}
	seq := childValue(r, "guides")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return r, nil, nil
	}
	return r, seq, nil
}

// documentRoot strips the DocumentNode wrapper that yaml.v3 always emits at
// the top of a parsed file, returning the actual content node (usually a
// MappingNode for our specs).
func documentRoot(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

// childValue returns the value node for key inside a MappingNode, or nil if the
// key isn't present. The content layout is [key, value, key, value, ...].
func childValue(m *yaml.Node, key string) *yaml.Node {
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// findGuideByID walks a `guides:` SequenceNode and returns the MappingNode for
// the guide whose `id:` matches.
func findGuideByID(seq *yaml.Node, id string) *yaml.Node {
	for _, g := range seq.Content {
		if g.Kind != yaml.MappingNode {
			continue
		}
		if idNode := childValue(g, "id"); idNode != nil && idNode.Value == id {
			return g
		}
	}
	return nil
}

// setOrAppendScalar updates an existing key's scalar value in m, or appends
// the key with the given value when absent. Style is chosen for the NEW value
// to keep YAML readable: block-literal for multi-line (preserves linebreaks
// as they are), single-quoted for non-ASCII (avoids "📤" → "\U0001F4E4"
// Unicode-escaping that yaml.v3's auto-style forces in double-quoted form),
// and yaml.v3's auto choice for plain ASCII single-line values.
func setOrAppendScalar(m *yaml.Node, key, value string) {
	style := scalarStyleFor(value)
	if v := childValue(m, key); v != nil {
		v.Value = value
		v.Style = style
		if v.Tag != "" && v.Tag != "!!str" {
			v.Tag = ""
		}
		return
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: style}
	m.Content = append(m.Content, keyNode, valueNode)
}

// scalarStyleFor returns the yaml.v3 emit style that gives the cleanest
// human-readable output for v.
func scalarStyleFor(v string) yaml.Style {
	if strings.ContainsRune(v, '\n') {
		return yaml.LiteralStyle
	}
	for _, r := range v {
		if r > 127 {
			return yaml.SingleQuotedStyle
		}
	}
	return 0
}

// writeFileAtomic writes data to path via a same-directory tempfile + rename.
// fsyncs both the file (so its data blocks reach disk before rename) and the
// containing directory (so the rename itself is durable), which closes the
// crash window where rename completes but the file ends up zero-length on
// next boot — the YAML this writes IS the docs-generator source-of-truth,
// so a torn write would surface as broken docs after a power loss.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cms-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	// Sync the directory entry so the rename is durable across crash.
	// Open/Sync errors are logged but not propagated — many platforms can't
	// open a dir for fsync (Windows, some FUSE backends), and a fsync error
	// after the rename means the file is still on disk, just maybe not in
	// the resilient state we'd prefer. Silently swallowing would let
	// "claims-to-be-atomic" diverge from reality without a single log line.
	if d, err := os.Open(dir); err == nil {
		if syncErr := d.Sync(); syncErr != nil {
			slog.Warn("writeFileAtomic: dir fsync failed", "dir", dir, "err", syncErr)
		}
		_ = d.Close()
	} else if !os.IsNotExist(err) {
		slog.Warn("writeFileAtomic: dir open for fsync failed", "dir", dir, "err", err)
	}
	return nil
}

// EditableFields is exported so the edit template can iterate (or we can
// surface it to clients) without re-declaring the list.
func EditableFields() []string {
	out := make([]string, len(editableFields))
	copy(out, editableFields)
	return out
}
