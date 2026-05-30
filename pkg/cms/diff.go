package cms

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// DiffLine is one rendered row in the preview's unified diff view. Kind tells
// the template which CSS class to apply (context / addition / removal / hunk
// header).
type DiffLine struct {
	Kind string // "ctx" | "add" | "del" | "hdr"
	Text string
}

// DiffStats summarises an edit at a glance — "+3 / −2".
type DiffStats struct {
	Added   int
	Removed int
}

// DiffYAML renders a unified diff between two YAML byte streams as a slice of
// classified DiffLines plus aggregate stats. Context size is 3 lines, matching
// GNU diff -u defaults so the output reads familiarly to engineers. Files that
// are byte-identical produce an empty diff and (0, 0) stats — the caller
// surfaces "no changes" in that case.
func DiffYAML(before, after []byte, beforeLabel, afterLabel string) ([]DiffLine, DiffStats, error) {
	u := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(before)),
		B:        difflib.SplitLines(string(after)),
		FromFile: beforeLabel,
		ToFile:   afterLabel,
		Context:  3,
	}
	raw, err := difflib.GetUnifiedDiffString(u)
	if err != nil {
		return nil, DiffStats{}, err
	}
	if raw == "" {
		return nil, DiffStats{}, nil
	}
	return classifyDiff(raw), countDiff(raw), nil
}

// classifyDiff turns a unified-diff string into structured DiffLines. We strip
// the trailing newline difflib leaves on each entry so the template can wrap
// each line in its own row without leaking blank lines.
func classifyDiff(s string) []DiffLine {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]DiffLine, 0, len(lines))
	for _, line := range lines {
		var kind string
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			kind = "hdr"
		case strings.HasPrefix(line, "@@"):
			kind = "hdr"
		case strings.HasPrefix(line, "+"):
			kind = "add"
		case strings.HasPrefix(line, "-"):
			kind = "del"
		default:
			kind = "ctx"
		}
		out = append(out, DiffLine{Kind: kind, Text: line})
	}
	return out
}

// countDiff tallies the +/- lines (ignoring file headers). Used by the preview
// page header so the editor sees the size of the edit before reading the diff.
func countDiff(s string) DiffStats {
	var st DiffStats
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// file header, not a data line
		case strings.HasPrefix(line, "+"):
			st.Added++
		case strings.HasPrefix(line, "-"):
			st.Removed++
		}
	}
	return st
}
