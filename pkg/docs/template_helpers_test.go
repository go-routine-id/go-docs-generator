package docs

import (
	"strings"
	"testing"
)

func TestSectionBaseURLs_SectionWins(t *testing.T) {
	info := InfoInfo{BaseURLs: []BaseURL{{Label: "Global", URL: "https://global"}}}
	sec := SectionInfo{BaseURLs: []BaseURL{{Label: "Acct", URL: "https://acct"}}}

	got := sectionBaseURLs(sec, info)
	if len(got) != 1 || got[0].URL != "https://acct" {
		t.Errorf("expected section override, got %+v", got)
	}
}

func TestSectionBaseURLs_FallbackToGlobal(t *testing.T) {
	info := InfoInfo{BaseURLs: []BaseURL{{Label: "Global", URL: "https://global"}}}
	sec := SectionInfo{}

	got := sectionBaseURLs(sec, info)
	if len(got) != 1 || got[0].URL != "https://global" {
		t.Errorf("expected fallback to Info.BaseURLs, got %+v", got)
	}
}

func TestSectionDefaultURL_Precedence(t *testing.T) {
	info := InfoInfo{
		BaseURL:  "https://info-default",
		BaseURLs: []BaseURL{{URL: "https://info-a"}, {URL: "https://info-default-env", Default: true}},
	}

	tests := []struct {
		name string
		sec  SectionInfo
		want string
	}{
		{"single override wins", SectionInfo{BaseURL: "https://sec-single"}, "https://sec-single"},
		{"section default env wins over single", SectionInfo{
			BaseURL:  "https://sec-single",
			BaseURLs: []BaseURL{{URL: "https://sec-def", Default: true}},
		}, "https://sec-single"}, // BaseURL (single) takes precedence
		{"section BaseURLs default used", SectionInfo{
			BaseURLs: []BaseURL{{URL: "https://sec-first"}, {URL: "https://sec-def", Default: true}},
		}, "https://sec-def"},
		{"section BaseURLs first if no default", SectionInfo{
			BaseURLs: []BaseURL{{URL: "https://sec-first"}, {URL: "https://sec-second"}},
		}, "https://sec-first"},
		{"fallback to info default env", SectionInfo{}, "https://info-default-env"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sectionDefaultURL(tc.sec, info)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSectionDefaultURL_FallbackInfoBaseURL(t *testing.T) {
	info := InfoInfo{BaseURL: "https://only-info"}
	if got := sectionDefaultURL(SectionInfo{}, info); got != "https://only-info" {
		t.Errorf("expected fallback to info.BaseURL when no lists, got %q", got)
	}
}

func TestSectionUsesGlobal(t *testing.T) {
	if !sectionUsesGlobal(SectionInfo{}) {
		t.Error("empty section should use global")
	}
	if sectionUsesGlobal(SectionInfo{BaseURL: "x"}) {
		t.Error("section with BaseURL should not use global")
	}
	if sectionUsesGlobal(SectionInfo{BaseURLs: []BaseURL{{URL: "x"}}}) {
		t.Error("section with BaseURLs should not use global")
	}
}

func TestRender_Theme(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Default Title
theme:
  title: Custom Docs
  logo_icon: "🚀"
  primary_color: "#ff0099"
  favicon: /my-favicon.ico
`)
	h, err := NewHandler(dir+"/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)

	checks := []string{
		"<title>Custom Docs</title>",            // Theme.Title overrides
		"🚀",                                    // Theme.LogoIcon
		`href="/my-favicon.ico"`,                // Theme.Favicon
		"--primary: #ff0099",                    // Theme.PrimaryColor CSS override
	}
	for _, needle := range checks {
		if !strings.Contains(got, needle) {
			t.Errorf("expected output to contain %q", needle)
		}
	}
	if strings.Contains(got, "<title>Default Title</title>") {
		t.Error("theme title should replace Info.Title, but Info.Title still appears")
	}
}

// TestRender_Events verifies that async EventChannel entries show up in both
// the sidebar and the main content area.
func TestRender_Events(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Event-Driven Service
events:
  - id: user-signup
    title: User Signup
    description: Fired when a user completes registration
    protocol: kafka
    address: user.signup.v1
    operations:
      - type: publish
        summary: New user signed up
        payload:
          - name: user_id
            type: string
            required: true
`)
	h, err := NewHandler(dir+"/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)

	checks := []string{
		"📡 Events",             // sidebar header
		"User Signup",           // title
		"user.signup.v1",        // address rendered
		`id="panel-event-0"`,    // panel id for first event
		"publish",               // operation type shown
	}
	for _, needle := range checks {
		if !strings.Contains(got, needle) {
			t.Errorf("expected output to contain %q", needle)
		}
	}
}

// TestRender_PerSectionBaseURL renders an end-to-end spec where two sections
// have different base URLs and verifies the rendered HTML carries both.
func TestRender_PerSectionBaseURL(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "index.yaml", `
info:
  title: Multi Service
  version: "1.0"
  base_url: https://default.example
  base_urls:
    - label: Prod
      url: https://default.example
      default: true
sections:
  - id: account
    title: Account
    base_url: https://account.example
    base_urls:
      - label: Account-Prod
        url: https://account.example
        default: true
      - label: Account-Staging
        url: https://staging.account.example
    endpoints:
      - name: Login
        method: POST
        path: /login
        auth: none
        description: Login endpoint
  - id: storage
    title: Storage
    base_url: https://storage.example
    endpoints:
      - name: Upload
        method: POST
        path: /upload
        auth: JWT
        description: Upload file
`)
	h, err := NewHandler(dir+"/index.yaml", false)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	out, err := h.Render("")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)

	checks := []string{
		`https://account.example/login`,       // section override applied to endpoint URL
		`https://storage.example/upload`,      // second section with different base_url
		`https://staging.account.example`,     // section-specific environment label
		`data-uses-global="false"`,            // section-level means not global
	}
	for _, needle := range checks {
		if !strings.Contains(got, needle) {
			t.Errorf("expected output to contain %q but it did not", needle)
		}
	}
}
