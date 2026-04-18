package docs

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "overwrite golden files with current render output")

// TestRender_Museum renders the Museum example project and compares to a stored
// golden HTML file. The fixture lives under testdata/ so it is decoupled from
// examples/ (which users may freely edit or delete).
// Regenerate the golden with: go test ./pkg/docs -run TestRender -update-golden
func TestRender_Museum(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", "museum.html")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s (%d bytes)", goldenPath, len(got))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v\nhint: run `go test ./pkg/docs -run TestRender -update-golden` to create it", goldenPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("render output differs from golden (len got=%d want=%d).\nhint: if the change is intentional, regenerate with `go test ./pkg/docs -run TestRender -update-golden` and review the diff in git.\nfirst divergence: %s", len(got), len(want), firstDivergence(got, want))
	}
}

// TestRender_StructuralInvariants guards core structural expectations that must
// hold regardless of spec content — ensures we never ship broken HTML.
func TestRender_StructuralInvariants(t *testing.T) {
	h, err := NewHandler("testdata/specs/museum/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	got, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(got)

	invariants := []struct {
		name  string
		check func(string) bool
	}{
		{"starts with doctype", func(s string) bool { return strings.HasPrefix(s, "<!DOCTYPE html>") }},
		{"ends with </html>", func(s string) bool { return strings.HasSuffix(strings.TrimSpace(s), "</html>") }},
		{"has <style>", func(s string) bool { return strings.Contains(s, "<style>") }},
		{"has </style>", func(s string) bool { return strings.Contains(s, "</style>") }},
		{"has babel script", func(s string) bool { return strings.Contains(s, `<script type="text/babel">`) }},
		{"has plain script", func(s string) bool { return strings.Contains(s, "<script>") }},
		{"has body", func(s string) bool { return strings.Contains(s, "<body>") && strings.Contains(s, "</body>") }},
		{"no unresolved template tags", func(s string) bool { return !strings.Contains(s, "{{") && !strings.Contains(s, "}}") }},
	}

	for _, inv := range invariants {
		if !inv.check(out) {
			t.Errorf("invariant failed: %s", inv.name)
		}
	}
}

// firstDivergence returns a short description of where two byte slices first differ.
func firstDivergence(a, b []byte) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			start := i - 40
			if start < 0 {
				start = 0
			}
			end := i + 40
			if end > len(a) {
				end = len(a)
			}
			return "byte " + itoa(i) + " — got: " + strings.ReplaceAll(string(a[start:end]), "\n", "\\n")
		}
	}
	return "length mismatch (common prefix equal)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
