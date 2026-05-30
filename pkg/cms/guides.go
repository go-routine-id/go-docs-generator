package cms

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	if isMultiDoc(raw) {
		return nil, fmt.Errorf("%s: multi-document YAML files are not supported by the CMS (split into one document per file)", path)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	root := documentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, nil
	}
	seq := childValue(root, "guides")
	if seq == nil || seq.Kind != yaml.SequenceNode {
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
		out = append(out, entry)
	}
	return out, nil
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
	if isMultiDoc(raw) {
		return nil, fmt.Errorf("%s: multi-document YAML files are not supported by the CMS (split into one document per file)", path)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	root := documentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top-level YAML is not a mapping", path)
	}
	seq := childValue(root, "guides")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: no top-level `guides:` sequence", path)
	}

	target := findGuideByID(seq, update.ID)
	if target == nil {
		return nil, ErrGuideNotFound
	}

	setOrAppendScalar(target, "icon", update.Icon)
	setOrAppendScalar(target, "title", update.Title)
	setOrAppendScalar(target, "description", update.Description)

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

// isMultiDoc returns true when raw contains more than one YAML document. yaml.v3
// silently ignores everything past the first DocumentNode, so DiscoverGuides
// and ProposedGuideYAML would be invisible to (and would corrupt edits of)
// guides living past a `---` separator. Rather than partially support it, we
// reject the file with a clear error and ask the author to split it.
func isMultiDoc(raw []byte) bool {
	d := yaml.NewDecoder(bytes.NewReader(raw))
	count := 0
	for {
		var n yaml.Node
		if err := d.Decode(&n); err != nil {
			break
		}
		count++
		if count > 1 {
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
func unescapeNonASCIIInDQ(span []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(span))
	for i := 0; i < len(span); i++ {
		c := span[i]
		if c == '\\' && i+1 < len(span) {
			nx := span[i+1]
			width := 0
			switch nx {
			case 'u':
				width = 4
			case 'U':
				width = 8
			}
			if width > 0 && i+2+width <= len(span) {
				if r, ok := parseHexRune(span[i+2 : i+2+width]); ok && r >= 0x80 {
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
	// Non-fatal on platforms where opening the dir fails (e.g. some Windows
	// configs) — best effort.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
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
