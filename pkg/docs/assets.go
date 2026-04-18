package docs

import "embed"

// vendorFS embeds the client-side dependencies (React, ReactFlow, dagre)
// served at {prefix}/assets/vendor/. Self-hosting these files ships the docs
// page without any CDN dependency — important for:
//   - strict CSP environments that block third-party script sources
//   - air-gapped / internal-only deployments
//   - deterministic behaviour (no version drift, no 404s when a CDN rotates)
//
// Regenerate by running: make vendor
//
//go:embed assets/vendor/*
var vendorFS embed.FS
