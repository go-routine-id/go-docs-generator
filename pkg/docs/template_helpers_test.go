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
		"<title>Custom Docs</title>", // Theme.Title overrides
		"🚀",                          // Theme.LogoIcon
		`href="/my-favicon.ico"`,     // Theme.Favicon
		"--primary: #ff0099",         // Theme.PrimaryColor CSS override
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
		"📡 Events",           // sidebar header
		"User Signup",        // title
		"user.signup.v1",     // address rendered
		`id="panel-event-0"`, // panel id for first event
		"publish",            // operation type shown
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
		`https://account.example/login`,   // section override applied to endpoint URL
		`https://storage.example/upload`,  // second section with different base_url
		`https://staging.account.example`, // section-specific environment label
		`data-uses-global="false"`,        // section-level means not global
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
		{
			// A '&' in the query string must be HTML-escaped exactly once —
			// `&amp;`, never the double-escaped `&amp;amp;`.
			"ampersand in query string escaped once",
			`open [link](https://x.com/a?b=1&c=2) ok`,
			`<a href="https://x.com/a?b=1&amp;c=2">link</a>`,
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

// TestInlineFmt_URLWithEmphasisChars guards against a rendering bug where
// URLs containing characters that double as Markdown emphasis delimiters
// (`*`, `_`, backtick) were getting wrapped in <em>/<code> tags INSIDE the
// href attribute, producing invalid HTML like
// `<a href="https://a.com/<em>x</em>">`. The fix is to extract links to
// placeholders before the emphasis pass.
func TestInlineFmt_URLWithEmphasisChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"asterisk in URL",
			`See [docs](https://example.com/*ref*) for more`,
			`<a href="https://example.com/*ref*">docs</a>`,
		},
		{
			"double asterisk in URL",
			`Read [it](https://example.com/**bold**)`,
			`<a href="https://example.com/**bold**">it</a>`,
		},
		{
			"backtick in URL",
			"Try [api](https://example.com/foo`bar)",
			"<a href=\"https://example.com/foo`bar\">api</a>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(mdInline(c.in))
			if !strings.Contains(got, c.want) {
				t.Errorf("input %q\n got: %s\nwant containing: %s", c.in, got, c.want)
			}
			// Belt-and-brace: no emphasis tags should appear INSIDE href.
			if strings.Contains(got, `href="`) && strings.Contains(got, `<em>`) && strings.Contains(got[strings.Index(got, `href="`):strings.Index(got, `">`)], `<em>`) {
				t.Errorf("emphasis tag leaked into href attribute: %s", got)
			}
		})
	}
}

// TestInlineFmt_CodeSpanAndUnclosed guards two emphasis-pass bugs: a `*`
// inside a code span used to be turned into <em> (because the * pass ran
// before the backtick pass), and an unclosed `**` produced stray
// <em></em> tags.
func TestInlineFmt_CodeSpanAndUnclosed(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string // substring that MUST appear
		notWant string // substring that must NOT appear ("" to skip)
	}{
		{
			"asterisk inside code span",
			"use `a*b` here",
			"<code>a*b</code>",
			"<em>",
		},
		{
			"code span then italic",
			"`x*y` and *real*",
			"<code>x*y</code>",
			"", // the *real* still becomes <em>real</em>; just don't mangle the code
		},
		{
			"unclosed bold does not emit empty em",
			"an unclosed **bold start",
			"bold start",
			"<em></em>",
		},
		{
			"backtick without close stays literal",
			"a stray ` backtick",
			"stray ` backtick",
			"<code>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(mdInline(c.in))
			if !strings.Contains(got, c.want) {
				t.Errorf("input %q\n got: %s\nwant containing: %s", c.in, got, c.want)
			}
			if c.notWant != "" && strings.Contains(got, c.notWant) {
				t.Errorf("input %q\n got: %s\nmust NOT contain: %s", c.in, got, c.notWant)
			}
		})
	}
	// The italic in "code span then italic" should still render.
	if got := string(mdInline("`x*y` and *real*")); !strings.Contains(got, "<em>real</em>") {
		t.Errorf("italic outside code span should still render: %s", got)
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

// TestInlineFmt_TripleEmphasis locks in well-formed nesting for `***both***`.
// Running the bold pass before a dedicated triple pass left a stray lone `*`
// that the italic pass mis-paired, producing crossed tags
// (`<strong><em>…</strong></em>`). Output must nest correctly and never cross.
func TestInlineFmt_TripleEmphasis(t *testing.T) {
	got := string(mdInline(`nested ***all three*** done`))
	want := `nested <strong><em>all three</em></strong> done`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "</strong></em>") {
		t.Errorf("crossed emphasis tags in output: %s", got)
	}
}

// TestMdToHTML_FencedCodeBlock guards the fenced-code-block path: an ASCII
// diagram inside ``` fences must render as a single verbatim <pre> block —
// newlines preserved, columns intact, HTML escaped — instead of being flowed
// into a paragraph (which mangled box-drawing diagrams like "Sequence at a
// glance" in the account guide).
func TestMdToHTML_FencedCodeBlock(t *testing.T) {
	src := "Intro line.\n\n```\n┌────┐\n│ A  │ <access>\n└────┘\n```\n\nOutro line."
	got := string(mdToHTML(src))

	if !strings.Contains(got, `<pre class="md-code"><code>`) {
		t.Fatalf("expected a md-code pre block; got:\n%s", got)
	}
	// Verbatim content with newlines preserved (not joined by spaces).
	if !strings.Contains(got, "┌────┐\n│ A  │") {
		t.Errorf("diagram newlines/columns not preserved; got:\n%s", got)
	}
	// Angle brackets inside the block must be HTML-escaped.
	if !strings.Contains(got, "&lt;access&gt;") {
		t.Errorf("expected escaped <access>; got:\n%s", got)
	}
	// The fence markers themselves must not leak into the output.
	if strings.Contains(got, "```") {
		t.Errorf("raw fence markers leaked into output:\n%s", got)
	}
	// Surrounding prose still renders as paragraphs.
	if !strings.Contains(got, "<p style=\"margin-bottom:0.75rem;\">Intro line.</p>") ||
		!strings.Contains(got, "<p style=\"margin-bottom:0.75rem;\">Outro line.</p>") {
		t.Errorf("surrounding prose not wrapped in paragraphs; got:\n%s", got)
	}
}

// TestMdToHTML_GFMTable guards the table path: a pipe table followed by a
// delimiter row must render as a real <table> with <th>/<td> cells (inline
// markdown applied), not collapse into a paragraph full of literal pipes and
// dashes — the bug that mangled the "provisioned audiences" table.
func TestMdToHTML_GFMTable(t *testing.T) {
	src := "Before.\n\n| Surface | Endpoint |\n|---|---|\n| Web | `/auth/google` |\n| iOS | *pending* |\n\nAfter."
	got := string(mdToHTML(src))

	if !strings.Contains(got, `<table class="md-table"><thead><tr><th>Surface</th><th>Endpoint</th></tr></thead>`) {
		t.Fatalf("table header not rendered; got:\n%s", got)
	}
	// Body cells with inline markdown applied (code span + italic).
	if !strings.Contains(got, "<td>Web</td><td><code>/auth/google</code></td>") {
		t.Errorf("code-span cell not rendered; got:\n%s", got)
	}
	if !strings.Contains(got, "<td>iOS</td><td><em>pending</em></td>") {
		t.Errorf("italic cell not rendered; got:\n%s", got)
	}
	// The delimiter row must not leak as literal dashes/pipes.
	if strings.Contains(got, "|---|") || strings.Contains(got, "| Surface |") {
		t.Errorf("raw table markup leaked into output:\n%s", got)
	}
	// Surrounding prose still becomes paragraphs.
	if !strings.Contains(got, ">Before.</p>") || !strings.Contains(got, ">After.</p>") {
		t.Errorf("surrounding prose not wrapped in paragraphs; got:\n%s", got)
	}
}

// TestMdToHTML_LanguageFenceHighlights guards the Chroma path: a language-tagged
// fence (```kotlin) must emit token <span>s under a <code class="chroma"> so the
// shipped CSS can colour it, while a bare fence stays plain so ASCII diagrams
// keep their exact characters and no spurious colouring.
func TestMdToHTML_LanguageFenceHighlights(t *testing.T) {
	got := string(mdToHTML("```kotlin\nval x = 1\n```"))
	if !strings.Contains(got, `<pre class="md-code"><code class="chroma">`) {
		t.Fatalf("expected highlighted chroma code block; got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="k">val</span>`) {
		t.Errorf("expected a keyword token span; got:\n%s", got)
	}

	// A bare fence must NOT get the chroma class — stays plain & verbatim.
	bare := string(mdToHTML("```\nval x = 1\n```"))
	if strings.Contains(bare, `class="chroma"`) {
		t.Errorf("bare fence must not be highlighted; got:\n%s", bare)
	}

	// An unknown language must also fall back to a plain block.
	unknown := string(mdToHTML("```nosuchlang\nhello\n```"))
	if strings.Contains(unknown, `class="chroma"`) {
		t.Errorf("unknown language must fall back to plain; got:\n%s", unknown)
	}
}

// TestChromaCSS verifies both palettes are emitted and the dark one is scoped
// under [data-theme="dark"] so a single highlighted block re-themes correctly.
func TestChromaCSS(t *testing.T) {
	css := string(chromaCSS())
	// Light palette must be scoped to non-dark so it can't bleed into dark mode
	// (github-dark omits some tokens and would otherwise inherit light colours).
	if !strings.Contains(css, `:root:not([data-theme="dark"]) .chroma .k {`) {
		t.Errorf("missing scoped light keyword rule; got:\n%s", css)
	}
	if !strings.Contains(css, `[data-theme="dark"] .chroma .k {`) {
		t.Errorf("missing dark-scoped keyword rule; got:\n%s", css)
	}
	// Chroma's own block background must be stripped (we own the bg).
	if strings.Contains(css, ".bg") {
		t.Errorf("chroma background rule leaked; got:\n%s", css)
	}
	// The Error token's solid-red background must be neutralised.
	if !strings.Contains(css, ".err { background: none; color: inherit; }") {
		t.Errorf("error token not neutralised; got:\n%s", css)
	}
}

func TestIsTableDelimiterRow(t *testing.T) {
	yes := []string{"|---|---|", "| --- | --- |", "|:--|--:|:-:|", "---|---"}
	no := []string{"| a | b |", "just prose", "| not | delim |", "------", ""}
	for _, s := range yes {
		if !isTableDelimiterRow(s) {
			t.Errorf("expected delimiter row: %q", s)
		}
	}
	for _, s := range no {
		if isTableDelimiterRow(s) {
			t.Errorf("did not expect delimiter row: %q", s)
		}
	}
}
