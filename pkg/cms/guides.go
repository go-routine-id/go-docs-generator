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
	"strconv"
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
//
// JSON tags are explicit (snake_case) so the wire format the drafts table
// stores is decoupled from Go field names. A future field rename in this
// struct won't silently invalidate every existing draft row.
type FlowStep struct {
	OrigKey string `json:"orig_key,omitempty"`
	// Title is required at every gate (form, draft, publish). Deliberately
	// NOT marked omitempty: an absent JSON field decodes as "" which the
	// guide-validation chain (collectFlowSteps, publish guard) rejects;
	// marking it omitempty would mask that signal during decode.
	Title           string `json:"title"`
	EndpointMethod  string `json:"endpoint_method,omitempty"`
	EndpointPath    string `json:"endpoint_path,omitempty"`
	CurlJWT         string `json:"curl_jwt,omitempty"`
	CurlAPIKey      string `json:"curl_api_key,omitempty"`
	ResponseExample string `json:"response_example,omitempty"`
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
//     Each key is consumed AT MOST ONCE — a duplicate OrigKey in updates
//     synthesises a fresh node for the second occurrence rather than
//     aliasing the same yaml.Node into two slots.
//   - update.OrigKey == "" → newly added step; synthesise a minimal mapping
//     with only the editable fields.
//   - Existing steps NOT referenced by any update → dropped (the editor
//     used the remove button).
//   - Output order follows `updates` so reordering works naturally.
//
// Steps that end up without a `step:` value get one assigned, starting from
// max(existing integer step values) + 1 and counting up. Using max+1 instead
// of position prevents collisions with author-chosen step values: inserting
// a new step in the middle of [step:1, step:2, step:3] used to write
// `step: 3` for the new row (because it's at array position 3), producing
// two YAML entries with `step: 3` and the docs page rendering two panels
// titled "Step 3:". max+1 guarantees the new step gets a value distinct
// from anything already in the flow.
func applyFlowEdits(guide *yaml.Node, updates []FlowStep) {
	if updates == nil {
		return
	}
	existing := childValue(guide, "flow")
	byKey := buildFlowKeyMap(existing)

	// Pre-pass: find the max integer step value across the existing nodes
	// the updates will reuse. Author values that don't parse as int (e.g.
	// "2a") are not counted — they coexist with the assigned integers.
	nextStep := maxReusedIntegerStep(updates, byKey) + 1

	newContent := make([]*yaml.Node, 0, len(updates))
	for _, u := range updates {
		var node *yaml.Node
		if u.OrigKey != "" {
			if n, ok := byKey[u.OrigKey]; ok {
				node = n
				delete(byKey, u.OrigKey) // consume so a duplicate falls through
			}
		}
		if node == nil {
			node = &yaml.Node{Kind: yaml.MappingNode}
		}
		applyFlowStep(node, u)
		if childValue(node, "step") == nil {
			appendIntScalar(node, "step", nextStep)
			nextStep++
		}
		newContent = append(newContent, node)
	}

	if existing != nil {
		existing.Content = newContent
		return
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "flow"}
	seqNode := &yaml.Node{Kind: yaml.SequenceNode, Content: newContent}
	guide.Content = append(guide.Content, keyNode, seqNode)
}

// maxReusedIntegerStep returns the max parseable integer `step:` value among
// the existing nodes the updates will actually reuse (matched via OrigKey).
// Existing nodes the editor REMOVED don't contribute — their step values are
// freed up, so the next assignment can reclaim them without collision. Same
// for non-integer author values like "2a" — they're skipped.
func maxReusedIntegerStep(updates []FlowStep, byKey map[string]*yaml.Node) int {
	max := 0
	for _, u := range updates {
		if u.OrigKey == "" {
			continue
		}
		n, ok := byKey[u.OrigKey]
		if !ok || n == nil {
			continue
		}
		v := childValue(n, "step")
		if v == nil {
			continue
		}
		if iv, err := strconv.Atoi(v.Value); err == nil && iv > max {
			max = iv
		}
	}
	return max
}

// buildFlowKeyMap walks a `flow:` sequence and returns a map from each
// row's disambiguated key (computed via disambigKey) to its *yaml.Node.
// The same algorithm runs on the READ side (flowStepsFromSeq), so a guide
// with duplicate `step:` values produces unique keys per occurrence on
// both sides — the round-trip preserves nodeB's nested fields instead of
// collapsing them onto nodeA.
func buildFlowKeyMap(existing *yaml.Node) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if existing == nil || existing.Kind != yaml.SequenceNode {
		return out
	}
	seen := map[string]int{}
	for i, n := range existing.Content {
		if n.Kind != yaml.MappingNode {
			continue
		}
		k := disambigKey(flowStepKey(n, i), seen)
		out[k] = n
	}
	return out
}

// disambigKey appends a `#N` occurrence suffix when `base` has already
// been seen, so duplicate `step:` values produce distinct round-trip keys.
// The counter is updated in place. The READ and WRITE sides MUST call this
// in the same iteration order over the same input to stay symmetric.
func disambigKey(base string, seen map[string]int) string {
	occ := seen[base]
	seen[base] = occ + 1
	if occ == 0 {
		return base
	}
	return base + "#" + strconv.Itoa(occ)
}

// applyFlowStep mutates one step mapping in place to match the editable
// fields of u. The `step:` value is NOT touched here — applyFlowEdits owns
// the assignment policy (max+1 for new steps; author labels preserved for
// existing) so it can see the full update list and avoid colliding with
// existing step values.
//
// If `endpoint:` exists but is NOT a MappingNode (e.g. `endpoint: null`),
// the value is REPLACED with a fresh MappingNode rather than mutated in
// place — yaml.v3 ignores Content on non-mappings at marshal time, so the
// editor's method/path edits would otherwise be silently discarded.
func applyFlowStep(node *yaml.Node, u FlowStep) {
	setOrAppendScalar(node, "title", u.Title)

	ep := childValue(node, "endpoint")
	if ep == nil {
		ep = &yaml.Node{Kind: yaml.MappingNode}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "endpoint"},
			ep,
		)
	} else if ep.Kind != yaml.MappingNode {
		fresh := &yaml.Node{Kind: yaml.MappingNode}
		if !replaceChildValue(node, "endpoint", fresh) {
			// Belt-and-braces: replaceChildValue must succeed here since
			// childValue just returned a non-nil ep, but if a future
			// refactor decouples the two, fall through to an append.
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "endpoint"},
				fresh,
			)
		}
		ep = fresh
	}
	setOrAppendScalar(ep, "method", u.EndpointMethod)
	setOrAppendScalar(ep, "path", u.EndpointPath)

	setScalarOptional(node, "curl_example_jwt", u.CurlJWT)
	setScalarOptional(node, "curl_example_api_key", u.CurlAPIKey)
	setScalarOptional(node, "response_example", u.ResponseExample)
}

// setScalarOptional handles a step's optional scalar field — three states:
//
//   - value != ""  → set or append the scalar (write the new value)
//   - value == "" and field absent → no-op (don't pollute the YAML with
//     empty keys the author never wrote)
//   - value == "" and field PRESENT → DELETE the field (editor cleared the
//     textarea intending to remove it; leaving a stale `field: ”` would
//     either render differently from absent or force the author to drop
//     into raw YAML to delete the key)
//
// Replaces the prior setScalarIfPresentOrNonEmpty which only covered the
// first two cases and left stale empties in the YAML on clear-to-delete.
func setScalarOptional(m *yaml.Node, key, value string) {
	if value != "" {
		setOrAppendScalar(m, key, value)
		return
	}
	deleteChild(m, key)
}

// appendIntScalar appends `key: <value>` to a MappingNode as a !!int-tagged
// scalar so the next yaml.Unmarshal targeting an `int` field parses it as
// a number. Plain decimal style.
func appendIntScalar(m *yaml.Node, key string, value int) {
	if m.Kind != yaml.MappingNode {
		return
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.Itoa(value),
	}
	m.Content = append(m.Content, keyNode, valueNode)
}

// deleteChild removes a key/value pair from a MappingNode. No-op when the
// key is absent or m isn't a MappingNode.
func deleteChild(m *yaml.Node, key string) {
	if m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// replaceChildValue swaps the value node for the named key in a MappingNode
// and reports whether the swap happened. Returns false when the key isn't
// present or m isn't a MappingNode — the previous void return invited
// future callers to assume the swap succeeded.
func replaceChildValue(m *yaml.Node, key string, newValue *yaml.Node) bool {
	if m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = newValue
			return true
		}
	}
	return false
}

// flowStepsFromSeq reads a `flow:` sequence into the editor's flat view.
// OrigKey is the textual `step:` value (e.g. "1", "2a") because that's how
// authors anchor their flows in prose; if absent we fall back to the index
// so the position is still round-trippable. Duplicate `step:` values get
// disambiguated with a `#N` suffix so each row has a UNIQUE OrigKey — the
// matching algorithm in buildFlowKeyMap on the write side uses the same
// counter so a guide with two `step: 1` rows round-trips both nodes
// faithfully instead of collapsing nodeB onto nodeA.
func flowStepsFromSeq(seq *yaml.Node) []FlowStep {
	out := make([]FlowStep, 0, len(seq.Content))
	seen := map[string]int{}
	for i, n := range seq.Content {
		if n.Kind != yaml.MappingNode {
			continue
		}
		step := FlowStep{OrigKey: disambigKey(flowStepKey(n, i), seen)}
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
// human-readable output for v while still preserving v as a STRING on
// re-parse. Values that resolve to YAML core-schema bool/null/int/float
// (like "true", "null", "42", "1.5") MUST be quoted; otherwise yaml.v3
// emits them plain and the next parse tags them as the wrong type,
// silently mutating a string field into a different scalar type.
func scalarStyleFor(v string) yaml.Style {
	if strings.ContainsRune(v, '\n') {
		return yaml.LiteralStyle
	}
	for _, r := range v {
		if r > 127 {
			return yaml.SingleQuotedStyle
		}
	}
	if mustQuoteToStayString(v) {
		return yaml.SingleQuotedStyle
	}
	return 0
}

// mustQuoteToStayString reports whether the YAML core schema would tag the
// emitted plain scalar as something other than !!str — i.e. we'd lose the
// string type if we emitted it unquoted. The list below is informed by
// yaml.v3's resolver: covers the canonical bool / null spellings, signed
// & unsigned ints in decimal/hex/octal/binary (via ParseInt base 0), all
// strconv-parseable floats, AND the YAML-specific float specials that
// strconv doesn't recognise (`.inf`, `.Inf`, `.INF`, `+.inf`, `-.inf`,
// `.nan`, `.NaN`, `.NAN`).
func mustQuoteToStayString(v string) bool {
	if v == "" {
		// An empty plain scalar is canonical YAML null on parse; quote to
		// keep it as the empty string.
		return true
	}
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	if isYAMLFloatSpecial(v) {
		return true
	}
	// Base 0 lets ParseInt accept `0x1F`, `0o17`, `0b101` in addition to
	// plain decimals — yaml.v3 tags all of those as !!int on parse, so
	// they would round-trip from string to int if emitted plain.
	if _, err := strconv.ParseInt(v, 0, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	return false
}

// isYAMLFloatSpecial covers the dotted-inf / dotted-nan literals YAML's
// core schema treats as !!float but strconv.ParseFloat doesn't recognise.
func isYAMLFloatSpecial(v string) bool {
	switch v {
	case ".inf", ".Inf", ".INF",
		"+.inf", "+.Inf", "+.INF",
		"-.inf", "-.Inf", "-.INF",
		".nan", ".NaN", ".NAN":
		return true
	}
	return false
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
