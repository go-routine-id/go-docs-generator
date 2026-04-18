package docs

import (
	"strings"
	"testing"
)

func TestLint_CleanSpec(t *testing.T) {
	spec := &APISpec{
		Info: InfoInfo{Title: "My API", Version: "1.0"},
		Sections: []SectionInfo{
			{ID: "users", Title: "Users", Description: "Manage users", Endpoints: []Endpoint{
				{Name: "List", Method: "GET", Path: "/users", Description: "List users"},
			}},
		},
	}
	if ds := Lint(spec); len(ds) != 0 {
		t.Errorf("clean spec produced diagnostics: %+v", ds)
	}
}

func TestLint_DuplicateSectionID(t *testing.T) {
	spec := &APISpec{
		Info: InfoInfo{Title: "X"},
		Sections: []SectionInfo{
			{ID: "dup", Title: "A", Description: "x", Endpoints: []Endpoint{{Name: "n", Method: "GET", Path: "/a", Description: "d"}}},
			{ID: "dup", Title: "B", Description: "x", Endpoints: []Endpoint{{Name: "n", Method: "GET", Path: "/b", Description: "d"}}},
		},
	}
	ds := Lint(spec)
	if !HasErrors(ds) {
		t.Fatal("expected error for duplicate id")
	}
	found := false
	for _, d := range ds {
		if d.Severity == SeverityError && strings.Contains(d.Message, "duplicate section id") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'duplicate section id' error, got: %+v", ds)
	}
}

func TestLint_RequiredFields(t *testing.T) {
	spec := &APISpec{
		// no Info.Title — error
		Sections: []SectionInfo{
			{ID: "s", Title: "S", Description: "x", Endpoints: []Endpoint{
				{Name: "n" /* missing method & path */, Description: "d"},
			}},
		},
	}
	ds := Lint(spec)
	if !HasErrors(ds) {
		t.Fatal("expected errors for missing required fields")
	}
	msgs := ""
	for _, d := range ds {
		msgs += d.String() + "\n"
	}
	for _, must := range []string{"info.title is required", "endpoint must have a method", "endpoint must have a path"} {
		if !strings.Contains(msgs, must) {
			t.Errorf("missing expected error: %q\ngot:\n%s", must, msgs)
		}
	}
}

func TestLint_DanglingFlowAnchor(t *testing.T) {
	spec := &APISpec{
		Info:     InfoInfo{Title: "X"},
		Sections: []SectionInfo{{ID: "s", Title: "S", Description: "x"}},
		Guides: []Guide{
			{ID: "g", Title: "G", Description: "d", Flow: []FlowStep{
				{Step: 1, Title: "t", Actions: []FlowAction{
					{Type: "link", Description: "go", Endpoint: "#does-not-exist"},
				}},
			}},
		},
	}
	ds := Lint(spec)
	if !HasErrors(ds) {
		t.Fatal("expected error for dangling anchor")
	}
	found := false
	for _, d := range ds {
		if strings.Contains(d.Message, "dangling anchor") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'dangling anchor' error, got: %+v", ds)
	}
}

func TestLint_AuthLabelConsistency(t *testing.T) {
	spec := &APISpec{
		Info: InfoInfo{Title: "X"},
		Sections: []SectionInfo{
			{ID: "s", Title: "S", Description: "x", Endpoints: []Endpoint{
				{Name: "a", Method: "GET", Path: "/a", Auth: "JWT", Description: "d"},
				{Name: "b", Method: "GET", Path: "/b", Auth: "jwt", Description: "d"},
				{Name: "c", Method: "GET", Path: "/c", Auth: "JWT Bearer", Description: "d"},
			}},
		},
	}
	ds := Lint(spec)
	found := false
	for _, d := range ds {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "inconsistent auth labels") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auth-consistency warning, got: %+v", ds)
	}
}

func TestLint_OrphanPermission(t *testing.T) {
	spec := &APISpec{
		Info: InfoInfo{Title: "X"},
		Permissions: []PermissionInfo{
			{Name: "users:read", Description: "r"},
		},
		Sections: []SectionInfo{
			{ID: "s", Title: "S", Description: "x", Endpoints: []Endpoint{
				{Name: "a", Method: "GET", Path: "/a", Permission: "users:read", Description: "d"},  // known
				{Name: "b", Method: "POST", Path: "/b", Permission: "users:write", Description: "d"}, // orphan
			}},
		},
	}
	ds := Lint(spec)
	found := false
	for _, d := range ds {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "users:write") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected orphan-permission warning for 'users:write', got: %+v", ds)
	}
}

// TestValidate_ExampleMuseum ensures the shipped example stays schema-valid —
// it's our canary if we change types.go without updating overrides.
func TestValidate_ExampleMuseum(t *testing.T) {
	errs := ValidateFile("testdata/specs/museum/index.yaml")
	if len(errs) > 0 {
		t.Errorf("Museum example failed schema validation:")
		for _, e := range errs {
			t.Errorf("  %s", e.Error())
		}
	}
}
