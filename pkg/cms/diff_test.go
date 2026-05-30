package cms

import (
	"strings"
	"testing"
)

func TestDiffYAML_IdenticalIsEmpty(t *testing.T) {
	a := []byte("guides:\n  - id: x\n    title: T\n")
	lines, stats, err := DiffYAML(a, a, "a", "b")
	if err != nil {
		t.Fatalf("DiffYAML: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("identical inputs should produce empty diff, got %d lines", len(lines))
	}
	if stats.Added != 0 || stats.Removed != 0 {
		t.Errorf("stats = %+v, want zero", stats)
	}
}

// TestDiffYAML_ClassifiesAndCounts is the headline test: an edit that changes
// one line must produce one addition + one removal, classified correctly so
// the template can color-code them, with the surrounding context preserved.
func TestDiffYAML_ClassifiesAndCounts(t *testing.T) {
	before := []byte("a\nb\nc\nd\ne\n")
	after := []byte("a\nb\nC\nd\ne\n")

	lines, stats, err := DiffYAML(before, after, "before", "after")
	if err != nil {
		t.Fatalf("DiffYAML: %v", err)
	}
	if stats.Added != 1 || stats.Removed != 1 {
		t.Errorf("stats = %+v, want +1/-1", stats)
	}

	// Walk the rendered lines and assert each kind appears at least once.
	kinds := map[string]int{}
	for _, l := range lines {
		kinds[l.Kind]++
	}
	for _, k := range []string{"hdr", "add", "del", "ctx"} {
		if kinds[k] == 0 {
			t.Errorf("missing diff kind %q in output: %+v", k, lines)
		}
	}

	// The removal line must reference the original `c`, the addition `C`.
	var foundDel, foundAdd bool
	for _, l := range lines {
		if l.Kind == "del" && strings.Contains(l.Text, "c") {
			foundDel = true
		}
		if l.Kind == "add" && strings.Contains(l.Text, "C") {
			foundAdd = true
		}
	}
	if !foundDel || !foundAdd {
		t.Errorf("expected -c / +C lines in diff, got: %+v", lines)
	}
}
