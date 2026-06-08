package docs

import (
	"html/template"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Chroma renders token <span>s with short class names (k, s, n, ...). We emit
// those classes (WithClasses) and ship the colour CSS ourselves so highlighting
// needs no inline styles and no JS — and so the same markup re-themes between
// light and dark via [data-theme].
var (
	chromaFormatter  = chromahtml.New(chromahtml.WithClasses(true), chromahtml.PreventSurroundingPre(true), chromahtml.TabWidth(2))
	chromaLightStyle = styles.Get("github")
	chromaDarkStyle  = styles.Get("github-dark")
)

// highlightCode returns syntax-highlighted inner HTML (token spans) for a
// fenced code block. ok=false when the language is empty or unknown, so the
// caller falls back to plain escaped text — this keeps the many bare ```
// ASCII-diagram fences uncoloured.
func highlightCode(lang, source string) (string, bool) {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "", false
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return "", false
	}
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return "", false
	}
	var buf strings.Builder
	if err := chromaFormatter.Format(&buf, chromaLightStyle, iterator); err != nil {
		return "", false
	}
	return buf.String(), true
}

// chromaCSS returns the token colour rules for both themes: the light palette as
// `.chroma .x { ... }` (applies by default) and the dark palette re-scoped under
// `[data-theme="dark"] .chroma .x`. Chroma's own block-background and
// pre-wrapper rules are dropped — our <pre class="md-code"> owns the background.
func chromaCSS() template.CSS {
	var light, dark strings.Builder
	_ = chromaFormatter.WriteCSS(&light, chromaLightStyle)
	_ = chromaFormatter.WriteCSS(&dark, chromaDarkStyle)

	// Both palettes must be scoped: github-dark omits some tokens (e.g.
	// punctuation) and leans on its base colour for them, so an unscoped light
	// rule like `.chroma .p` would otherwise bleed into dark mode and render
	// that token invisible. data-theme lives on <html>, default != "dark".
	var out strings.Builder
	out.WriteString(transformChromaCSS(light.String(), ".light", ":root:not([data-theme=\"dark\"]) .chroma"))
	out.WriteString(transformChromaCSS(dark.String(), ".dark", "[data-theme=\"dark\"] .chroma"))
	out.WriteString(".md-code code.chroma { background: none; }\n")
	return template.CSS(out.String())
}

// cssColorRe extracts a `color:` value while ignoring `background-color:`.
var cssColorRe = regexp.MustCompile(`(?:^|[^-])color:\s*([^;}]+)`)

// transformChromaCSS rewrites one theme's WriteCSS output: it strips the leading
// `/* Name */` comment from each rule, drops the block-background and
// pre-wrapper rules, and replaces the theme-scoped root selector
// (`.chroma.light` / `.chroma.dark`) with the desired prefix so token rules
// match our `<code class="chroma">` markup.
func transformChromaCSS(css, themeClass, replacement string) string {
	var b strings.Builder
	preSelector := ".chroma" + themeClass + " {"
	for _, line := range strings.Split(css, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "/*") {
			if idx := strings.Index(t, "*/"); idx != -1 {
				t = strings.TrimSpace(t[idx+2:])
			}
		}
		if t == "" {
			continue
		}
		// Drop the block-background (.bg...) rule — our <pre class="md-code">
		// owns the background.
		if strings.HasPrefix(t, ".bg") {
			continue
		}
		// The pre-wrapper rule also carries the base text colour (untokenised
		// text and tokens that inherit — e.g. github-dark punctuation — rely on
		// it). Keep just that colour; drop its background and vendor props.
		if strings.HasPrefix(t, preSelector) {
			if m := cssColorRe.FindStringSubmatch(t); m != nil {
				b.WriteString(replacement + " { color: " + strings.TrimSpace(m[1]) + "; }\n")
			}
			continue
		}
		// Neutralise the Error token: github gives `...`-style placeholders a
		// jarring solid-red background. Render them as ordinary text.
		if strings.HasPrefix(t, ".chroma"+themeClass+" .err ") {
			b.WriteString(replacement + " .err { background: none; color: inherit; }\n")
			continue
		}
		t = strings.ReplaceAll(t, ".chroma"+themeClass, replacement)
		b.WriteString(t + "\n")
	}
	return b.String()
}
