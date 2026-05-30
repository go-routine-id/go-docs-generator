package cms

import (
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
// exactly what publishing will do before they commit.
func ProposedGuideYAML(path string, update GuideEntry) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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
	// requested scalar style, which makes icons like "📤" round-trip as
	// `"\U0001F4E4"` — valid but ugly to read in the file. Convert them
	// back to UTF-8 in the wire form. Literal-backslash escapes
	// (`\\U…`) are skipped so we don't corrupt content authored that way.
	return unescapeUnicode(out), nil
}

// unescapeUnicode rewrites \uXXXX and \UXXXXXXXX escape sequences in b back
// into the UTF-8 bytes of the rune they represent — but only when the
// preceding byte isn't itself a backslash (which would mean an escaped
// backslash, not a Unicode escape).
func unescapeUnicode(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == '\\' && i+1 < len(b) && (i == 0 || b[i-1] != '\\') {
			width := 0
			switch b[i+1] {
			case 'u':
				width = 4
			case 'U':
				width = 8
			}
			if width > 0 && i+2+width <= len(b) {
				if r, ok := parseHexRune(b[i+2 : i+2+width]); ok {
					out = append(out, []byte(string(r))...)
					i += 1 + width
					continue
				}
			}
		}
		out = append(out, c)
	}
	return out
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

// writeFileAtomic writes data to path via a same-directory tempfile + rename
// so a crash mid-write can't leave a truncated YAML on disk.
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
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
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
