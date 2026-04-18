// Command gendocs regenerates the JSON Schema and SPEC.md for the APISpec type.
//
// Run: go run ./cmd/gendocs
//
// Outputs:
//   - schemas/spec.schema.json — JSON Schema Draft 2020-12 (for IDE autocomplete and validation)
//   - SPEC.md                  — human-readable reference generated from the schema
//
// Re-run whenever types in pkg/docs change.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"docs-generator/pkg/docs"

	"github.com/invopop/jsonschema"
)

func main() {
	reflector := &jsonschema.Reflector{
		ExpandedStruct:             true,  // inline top-level APISpec
		DoNotReference:             false, // preserve $defs for reuse
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: false,
	}

	schema := reflector.Reflect(&docs.APISpec{})

	// Write JSON schema to two places:
	//   - schemas/spec.schema.json (root, user-visible, referenced from YAML)
	//   - pkg/docs/spec.schema.json (co-located, embedded into the binary for validation)
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fatalf("marshal schema: %v", err)
	}
	schemaBytes = append(schemaBytes, '\n')
	if err := os.WriteFile("schemas/spec.schema.json", schemaBytes, 0o644); err != nil {
		fatalf("write root schema: %v", err)
	}
	if err := os.WriteFile("pkg/docs/spec.schema.json", schemaBytes, 0o644); err != nil {
		fatalf("write embedded schema: %v", err)
	}

	// Render SPEC.md from schema
	md := renderMarkdown(schema)
	if err := os.WriteFile("SPEC.md", []byte(md), 0o644); err != nil {
		fatalf("write SPEC.md: %v", err)
	}

	fmt.Fprintln(os.Stderr, "schemas/spec.schema.json and SPEC.md regenerated")
}

// renderMarkdown walks the schema and produces a section-per-type reference doc.
func renderMarkdown(root *jsonschema.Schema) string {
	var b strings.Builder
	b.WriteString("# Spec Reference\n\n")
	b.WriteString("> Auto-generated from Go structs in `pkg/docs`. **Do not edit by hand.**\n>\n")
	b.WriteString("> Regenerate with: `go run ./cmd/gendocs`\n\n")
	b.WriteString("This document describes the shape of a Docs Generator spec file (`spec/index.yaml` or equivalent).\n")
	b.WriteString("The same schema is also published as JSON Schema Draft 2020-12 in [`schemas/spec.schema.json`](schemas/spec.schema.json) ")
	b.WriteString("and can be referenced from YAML files with:\n\n")
	b.WriteString("```yaml\n# yaml-language-server: $schema=./schemas/spec.schema.json\n```\n\n")
	b.WriteString("> 💡 For a narrative guide — when to use each mode, worked monolith-vs-microservice examples, ")
	b.WriteString("conventions, and FAQ — see **[`docs/writing-specs.md`](docs/writing-specs.md)**.\n\n")

	b.WriteString("## Merge rules (multi-file specs)\n\n")
	b.WriteString("When a spec directory contains multiple YAML files, they are merged into a single document:\n\n")
	b.WriteString("- **Slice fields** (e.g. `sections`, `guides`, `screens`): appended — every file contributes.\n")
	b.WriteString("- **Nested object fields** (e.g. `info`): merged per-field — overlay non-zero values override.\n")
	b.WriteString("- **Scalar fields** (strings, numbers, booleans): overlay value wins when non-zero.\n\n")

	b.WriteString("## Top-level fields\n\n")
	writePropertyTable(&b, root)

	b.WriteString("\n## Nested types\n\n")
	defs := root.Definitions
	if defs != nil {
		names := make([]string, 0, len(defs))
		for name := range defs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			def := defs[name]
			b.WriteString("### `")
			b.WriteString(name)
			b.WriteString("`\n\n")
			if def.Description != "" {
				b.WriteString(def.Description)
				b.WriteString("\n\n")
			}
			writePropertyTable(&b, def)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func writePropertyTable(b *strings.Builder, s *jsonschema.Schema) {
	if s == nil || s.Properties == nil || s.Properties.Len() == 0 {
		b.WriteString("_No properties._\n")
		return
	}

	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}

	b.WriteString("| Field | Type | Required | Description |\n")
	b.WriteString("|-------|------|----------|-------------|\n")

	for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
		name := pair.Key
		val := pair.Value
		typeStr := schemaTypeString(val)
		desc := val.Description
		if desc == "" {
			desc = "—"
		}
		req := "no"
		if required[name] {
			req = "**yes**"
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", name, typeStr, req, escapePipes(desc))
	}
}

// schemaTypeString renders a Schema's type for a table cell.
func schemaTypeString(s *jsonschema.Schema) string {
	if s == nil {
		return "?"
	}
	if s.Ref != "" {
		return "[`" + refLastSegment(s.Ref) + "`](#" + strings.ToLower(refLastSegment(s.Ref)) + ")"
	}
	if s.Type == "array" {
		item := "?"
		if s.Items != nil {
			item = schemaTypeString(s.Items)
		}
		return "array<" + item + ">"
	}
	if s.Type == "" && s.OneOf != nil {
		parts := make([]string, 0, len(s.OneOf))
		for _, o := range s.OneOf {
			parts = append(parts, schemaTypeString(o))
		}
		return strings.Join(parts, " \\| ")
	}
	return "`" + s.Type + "`"
}

func refLastSegment(ref string) string {
	i := strings.LastIndex(ref, "/")
	if i < 0 {
		return ref
	}
	return ref[i+1:]
}

func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gendocs: "+format+"\n", args...)
	os.Exit(1)
}
