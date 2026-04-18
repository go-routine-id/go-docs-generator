package docs

import "embed"

// templateFS embeds the HTML templates used to render documentation.
// Templates are split into:
//   - docs.gohtml       — root page skeleton
//   - styles.gohtml     — CSS (define "styles")
//   - flow_diagram.gohtml — ReactFlow JSX (define "flow_diagram")
//   - api_tester.gohtml   — API tester JS (define "api_tester")
//
//go:embed templates/*.gohtml
var templateFS embed.FS
