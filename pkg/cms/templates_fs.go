package cms

import "embed"

// TemplateFS embeds the server-rendered HTML templates so the binary stays a
// single file. Vanilla HTML + a sprinkle of CSS — no build step, matches the
// docs-generator ethos.
//
//go:embed templates/*.gohtml
var TemplateFS embed.FS
