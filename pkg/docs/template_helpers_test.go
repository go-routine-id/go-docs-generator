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

// TestInlineFmt_MarkdownLinks asserts the `[text](url)` shorthand becomes a
// real anchor in all rendered descriptions and overview cards. Before this
// support was added the brackets and parens leaked into the page verbatim —
// the exact bug a multi-project hub spec hit when listing its sub-services.
func TestInlineFmt_MarkdownLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // a substring that MUST appear in the output
	}{
		{
			"absolute internal path",
			`See [Account Service](/docs?p=account) for details`,
			`<a href="/docs?p=account">Account Service</a>`,
		},
		{
			"https URL",
			`Hosted at [example](https://example.com).`,
			`<a href="https://example.com">example</a>`,
		},
		{
			"anchor link",
			`Jump to [overview](#overview).`,
			`<a href="#overview">overview</a>`,
		},
		{
			"mailto link",
			`Email [us](mailto:dev@example.com)`,
			`<a href="mailto:dev@example.com">us</a>`,
		},
		{
			"bold inside link text",
			`[**urgent** patch](https://example.com)`,
			`<a href="https://example.com"><strong>urgent</strong> patch</a>`,
		},
		{
			"multiple links in one line",
			`[A](/a) and [B](/b)`,
			`<a href="/a">A</a> and <a href="/b">B</a>`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(mdInline(c.in))
			if !strings.Contains(got, c.want) {
				t.Errorf("input %q\n got: %s\nwant containing: %s", c.in, got, c.want)
			}
		})
	}
}

// TestInlineFmt_LinkXSSGuard makes sure scheme whitelisting actually blocks
// the obvious attack vectors. javascript: and data: URLs must NOT produce an
// anchor — the original bracketed text is preserved verbatim so the author
// notices.
func TestInlineFmt_LinkXSSGuard(t *testing.T) {
	rejects := []string{
		`[click](javascript:alert(1))`,
		`[xss](JaVaScRiPt:alert(1))`,
		`[evil](data:text/html,<script>alert(1)</script>)`,
		`[also evil](vbscript:msgbox)`,
	}
	for _, in := range rejects {
		got := string(mdInline(in))
		if strings.Contains(got, "<a ") {
			t.Errorf("expected NO anchor tag for unsafe URL — input %q produced: %s", in, got)
		}
	}
}
