package docs

import (
	"os"
	"path/filepath"
	"testing"
)

// writeYAML creates a YAML file at the given relative path inside dir.
func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestLoadFileSpec_Minimal(t *testing.T) {
	dir := t.TempDir()
	p := writeYAML(t, dir, "spec.yaml", `
info:
  title: Test API
  version: "1.0"
sections:
  - id: s1
    title: Section 1
`)
	spec, err := loadFileSpec(p)
	if err != nil {
		t.Fatalf("loadFileSpec: %v", err)
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("title = %q, want %q", spec.Info.Title, "Test API")
	}
	if len(spec.Sections) != 1 || spec.Sections[0].ID != "s1" {
		t.Errorf("sections = %+v", spec.Sections)
	}
}

func TestLoadFileSpec_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	// Indentation that YAML parser rejects: child indented less than parent.
	p := writeYAML(t, dir, "bad.yaml", "info:\n  title: x\n y: z\n")
	if _, err := loadFileSpec(p); err == nil {
		t.Fatal("expected error for malformed YAML indentation, got nil")
	}
}

// TestLoadSpecFromPath_IndexAutoInclude verifies that pointing at index.yaml
// triggers directory mode (sibling files get merged).
func TestLoadSpecFromPath_IndexAutoInclude(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Root
sections:
  - id: root
    title: Root section
`)
	writeYAML(t, dir, "sections/extra.yaml", `
sections:
  - id: extra
    title: Extra section
`)
	spec, err := loadSpecFromPath(filepath.Join(dir, "index.yaml"))
	if err != nil {
		t.Fatalf("loadSpecFromPath: %v", err)
	}
	if len(spec.Sections) != 2 {
		t.Errorf("sections = %d, want 2 (merged). got: %+v", len(spec.Sections), spec.Sections)
	}
}

func TestLoadDirSpec_MergesArraysAppends(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Merged
sections:
  - id: a
    title: A
guides:
  - id: g1
    title: Guide 1
    flow: []
permissions:
  - name: p:read
    description: read
`)
	writeYAML(t, dir, "sections/b.yaml", `
sections:
  - id: b
    title: B
`)
	writeYAML(t, dir, "guides/g2.yaml", `
guides:
  - id: g2
    title: Guide 2
    flow: []
`)
	spec, err := loadDirSpec(dir)
	if err != nil {
		t.Fatalf("loadDirSpec: %v", err)
	}
	if len(spec.Sections) != 2 {
		t.Errorf("sections = %d, want 2", len(spec.Sections))
	}
	if len(spec.Guides) != 2 {
		t.Errorf("guides = %d, want 2", len(spec.Guides))
	}
	if len(spec.Permissions) != 1 {
		t.Errorf("permissions = %d, want 1", len(spec.Permissions))
	}
}

// TestMergeSpec_ScalarFieldwiseOverride documents the merge contract:
// overlay scalars override per-field when non-zero; zero scalars don't touch base.
func TestMergeSpec_ScalarFieldwiseOverride(t *testing.T) {
	base := &APISpec{}
	base.Info.Title = "Original"
	base.Info.Version = "1.0"

	overlay := &APISpec{}
	overlay.Info.Title = "Replaced" // wins (non-zero)
	// overlay.Info.Version is "" — must NOT clobber base.

	mergeSpec(base, overlay)

	if base.Info.Title != "Replaced" {
		t.Errorf("Title = %q, want Replaced", base.Info.Title)
	}
	if base.Info.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0 (zero overlay must not clobber base)", base.Info.Version)
	}
}

// TestMergeSpec_NoDuplicateNestedArrays is a regression guard for a bug in the
// prior hand-written mergeSpec that caused Info.OverviewCards to be duplicated
// whenever overlay had both Info and OverviewCards set.
func TestMergeSpec_NoDuplicateNestedArrays(t *testing.T) {
	base := &APISpec{}
	base.Info.Title = "A"
	base.Info.OverviewCards = []OverviewCard{{Title: "base-card"}}

	overlay := &APISpec{}
	overlay.Info.Title = "B"
	overlay.Info.OverviewCards = []OverviewCard{{Title: "overlay-card"}}

	mergeSpec(base, overlay)

	if len(base.Info.OverviewCards) != 2 {
		t.Fatalf("cards = %d, want 2 (base + overlay, no duplication). got: %+v", len(base.Info.OverviewCards), base.Info.OverviewCards)
	}
	titles := []string{base.Info.OverviewCards[0].Title, base.Info.OverviewCards[1].Title}
	if titles[0] != "base-card" || titles[1] != "overlay-card" {
		t.Errorf("unexpected merge order: %v", titles)
	}
}

func TestMergeSpec_EmptyOverlayIsNoOp(t *testing.T) {
	base := &APISpec{}
	base.Info.Title = "Kept"
	base.Sections = []SectionInfo{{ID: "x"}}

	mergeSpec(base, &APISpec{})

	if base.Info.Title != "Kept" {
		t.Errorf("empty overlay should not touch Info.Title, got %q", base.Info.Title)
	}
	if len(base.Sections) != 1 {
		t.Errorf("empty overlay should not affect sections, got %d", len(base.Sections))
	}
}

func TestDiscoverProjects_SubdirWithIndex(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Default
`)
	writeYAML(t, dir, "account/index.yaml", `
info:
  title: Account
`)
	writeYAML(t, dir, "storage/index.yaml", `
info:
  title: Storage
`)
	// A sub-directory without index.yaml should NOT become a project.
	writeYAML(t, dir, "notes/readme.yaml", `
info:
  title: ignored
`)

	projects, err := discoverProjects(dir)
	if err != nil {
		t.Fatalf("discoverProjects: %v", err)
	}

	// Expect: "" (default) + "account" + "storage". "notes" should be absent.
	if _, ok := projects[""]; !ok {
		t.Error("missing default project")
	}
	if p, ok := projects["account"]; !ok || p.Info.Title != "Account" {
		t.Errorf("account project missing or wrong: %+v", p)
	}
	if p, ok := projects["storage"]; !ok || p.Info.Title != "Storage" {
		t.Errorf("storage project missing or wrong: %+v", p)
	}
	if _, ok := projects["notes"]; ok {
		t.Error("notes should not be a project (no index.yaml)")
	}
}

// TestIsOpenAPIDocument exercises the detection heuristic, including the
// false-positive cases the previous implementation got wrong (substring
// `openapi:` appearing inside a description, or as a non-version value).
func TestIsOpenAPIDocument(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			"YAML OpenAPI 3.0",
			"openapi: 3.0.3\ninfo:\n  title: x",
			true,
		},
		{
			"YAML OpenAPI 3.1 quoted",
			`openapi: "3.1.0"` + "\ninfo:\n  title: x",
			true,
		},
		{
			"JSON OpenAPI 3.0",
			`{"openapi": "3.0.0", "info": {"title": "x"}}`,
			true,
		},
		{
			"docs-gen spec without openapi key",
			"info:\n  title: x\nsections:\n  - id: s1",
			false,
		},
		{
			"description mentioning `openapi:` at column 0",
			"info:\n  title: x\n  description: |\n    See openapi: 3.0 below\n",
			// Multi-line literal block: `openapi:` is at column 4, not 0. Should NOT match.
			false,
		},
		{
			"Swagger 2.0 (not OpenAPI 3.x)",
			"swagger: \"2.0\"\ninfo:\n  title: x",
			false,
		},
		{
			"value-only `openapi:` without version",
			"openapi:\ninfo:\n  title: x",
			false,
		},
		{
			"openapi: 4.x (future major) — must not match",
			"openapi: 4.0.0\n",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isOpenAPIDocument([]byte(c.body))
			if got != c.want {
				t.Errorf("isOpenAPIDocument = %v, want %v\nbody:\n%s", got, c.want, c.body)
			}
		})
	}
}

// TestMergeSpec_MergesSectionsByID guards the contract that two overlay
// files declaring sections with the same `id` collapse into one section
// with the union of their endpoints — not two sections with duplicate
// IDs and clashing HTML anchors.
func TestMergeSpec_MergesSectionsByID(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Demo
  version: "1.0"
`)
	writeYAML(t, dir, "sections/users-part1.yaml", `
sections:
  - id: users
    title: Users
    endpoints:
      - name: List
        method: GET
        path: /users
`)
	writeYAML(t, dir, "sections/users-part2.yaml", `
sections:
  - id: users
    endpoints:
      - name: Create
        method: POST
        path: /users
  - id: orders
    title: Orders
    endpoints:
      - name: List
        method: GET
        path: /orders
`)

	spec, err := loadDirSpec(dir)
	if err != nil {
		t.Fatalf("loadDirSpec: %v", err)
	}

	if got := len(spec.Sections); got != 2 {
		t.Fatalf("section count = %d, want 2 (users merged, orders added). got: %+v", got, spec.Sections)
	}

	var users, orders *SectionInfo
	for i := range spec.Sections {
		s := &spec.Sections[i]
		switch s.ID {
		case "users":
			users = s
		case "orders":
			orders = s
		}
	}
	if users == nil || orders == nil {
		t.Fatalf("expected sections users + orders, got %+v", spec.Sections)
	}
	if users.Title != "Users" {
		t.Errorf("users.Title = %q, want %q (overlay must not blank out the base title)", users.Title, "Users")
	}
	if got := len(users.Endpoints); got != 2 {
		t.Errorf("users.Endpoints count = %d, want 2 (List from part1 + Create from part2)", got)
	}
	if got := len(orders.Endpoints); got != 1 {
		t.Errorf("orders.Endpoints count = %d, want 1", got)
	}
}

// TestMergeSpec_AppendsUnkeyedSlices verifies the non-keyed fallback still
// appends. Endpoints have no ID and so should append within their containing
// section's merge.
func TestMergeSpec_AppendsUnkeyedSlices(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Demo
sections:
  - id: a
    title: A
    endpoints:
      - name: One
        method: GET
        path: /one
`)
	writeYAML(t, dir, "overlay.yaml", `
sections:
  - id: a
    endpoints:
      - name: Two
        method: GET
        path: /two
`)
	spec, err := loadDirSpec(dir)
	if err != nil {
		t.Fatalf("loadDirSpec: %v", err)
	}
	if len(spec.Sections) != 1 {
		t.Fatalf("expected 1 section after id-merge, got %d", len(spec.Sections))
	}
	got := len(spec.Sections[0].Endpoints)
	if got != 2 {
		t.Errorf("merged endpoints = %d, want 2", got)
	}
}
