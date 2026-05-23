package docs

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed spec.schema.json
var embeddedSchemaJSON []byte

// ValidationError carries a single schema-level violation, with enough context
// (file + JSON path) for a human to find the offending line.
type ValidationError struct {
	File    string `json:"file,omitempty"`
	Path    string `json:"path,omitempty"` // e.g. ".sections[0].endpoints[1].method"
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidateFile loads a YAML file (or merges a directory) and validates it
// against the embedded JSON Schema. OpenAPI inputs are skipped — they're
// already validated upstream by the kin-openapi loader. Returns nil on
// success, or a slice of ValidationError for every violation.
//
// Validation runs against the RAW parsed YAML of each contributing source
// file, not the round-tripped Go struct. This matters: unmarshalling into
// APISpec silently drops unknown keys, so validating the struct can never
// catch a misspelled field (`sectionz:`, `methdo:`). Validating the raw
// YAML lets the schema's `additionalProperties: false` do its job — typo
// detection is the single most useful thing this command offers a spec
// author. Each overlay file is checked independently because the schema
// permits partial documents (any top-level field may be omitted).
func ValidateFile(path string) []ValidationError {
	info, err := os.Stat(path)
	if err != nil {
		return []ValidationError{{File: path, Message: err.Error()}}
	}

	target := path
	if info.IsDir() {
		target = filepath.Join(path, "index.yaml")
	}

	files, err := sourceYAMLFiles(target)
	if err != nil {
		return []ValidationError{{File: target, Message: err.Error()}}
	}

	schema, err := compileSchema()
	if err != nil {
		return []ValidationError{{File: target, Message: err.Error()}}
	}

	var out []ValidationError
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			out = append(out, ValidationError{File: f, Message: err.Error()})
			continue
		}
		// Skip OpenAPI inputs — they already passed the OpenAPI loader and
		// have a different shape than our native schema.
		if isOpenAPIDocument(raw) {
			continue
		}
		doc, err := decodeYAMLToJSON(f)
		if err != nil {
			out = append(out, ValidationError{File: f, Message: "parse yaml: " + err.Error()})
			continue
		}
		if doc == nil {
			continue // empty/comment-only overlay — nothing to validate
		}
		if err := schema.Validate(doc); err != nil {
			out = append(out, flattenSchemaErrors(f, err)...)
		}
	}
	return out
}

// sourceYAMLFiles returns every YAML file that contributes to the spec at
// target, mirroring loadDirSpec's traversal: a single file yields itself; an
// index.yaml (or directory) yields index.yaml plus every sibling overlay,
// skipping subdirectories that are themselves projects (own index.yaml).
func sourceYAMLFiles(target string) ([]string, error) {
	base := filepath.Base(target)
	if !strings.EqualFold(base, "index.yaml") {
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			return []string{target}, nil
		}
	}

	dir := filepath.Dir(target)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		dir = target
	}
	indexPath := filepath.Join(dir, "index.yaml")

	var files []string
	if _, err := os.Stat(indexPath); err == nil {
		files = append(files, indexPath)
	}
	walkErr := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir {
				if _, statErr := os.Stat(filepath.Join(p, "index.yaml")); statErr == nil {
					return filepath.SkipDir // separate project
				}
			}
			return nil
		}
		if p == indexPath {
			return nil
		}
		lower := strings.ToLower(p)
		if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
			files = append(files, p)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan spec directory %s: %w", dir, walkErr)
	}
	sort.Strings(files)
	return files, nil
}

// compileSchema compiles the embedded JSON Schema once for reuse.
func compileSchema() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("spec.schema.json", bytes.NewReader(embeddedSchemaJSON)); err != nil {
		return nil, fmt.Errorf("load embedded schema: %w", err)
	}
	schema, err := compiler.Compile("spec.schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}
	return schema, nil
}

// ValidateSpec validates an in-memory APISpec against the embedded schema.
// file is used only for error reporting and may be empty.
//
// Note: because the input is an already-parsed struct, this path cannot
// detect unknown/misspelled keys (they were dropped during unmarshal). Use
// ValidateRaw / ValidateFile when the original bytes are available and you
// want `additionalProperties` enforcement.
func ValidateSpec(spec *APISpec, file string) []ValidationError {
	doc, err := specAsJSONValue(spec)
	if err != nil {
		return []ValidationError{{File: file, Message: "marshal spec to json: " + err.Error()}}
	}
	schema, err := compileSchema()
	if err != nil {
		return []ValidationError{{File: file, Message: err.Error()}}
	}
	if err := schema.Validate(doc); err != nil {
		return flattenSchemaErrors(file, err)
	}
	return nil
}

// ValidateRaw validates raw YAML/JSON bytes against the schema, preserving
// unknown-key detection (the struct round-trip in ValidateSpec cannot). Used
// by the HTTP /validate endpoint where the original request body is in hand.
func ValidateRaw(raw []byte, file string) []ValidationError {
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return []ValidationError{{File: file, Message: "parse: " + err.Error()}}
	}
	doc = normalizeYAMLValue(doc)
	if doc == nil {
		return nil
	}
	schema, err := compileSchema()
	if err != nil {
		return []ValidationError{{File: file, Message: err.Error()}}
	}
	if err := schema.Validate(doc); err != nil {
		return flattenSchemaErrors(file, err)
	}
	return nil
}

// specAsJSONValue marshals APISpec to JSON and unmarshals it back into a
// generic any value — the form jsonschema.Validate expects.
func specAsJSONValue(spec *APISpec) (any, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// flattenSchemaErrors turns the tree-shaped error from jsonschema into a flat
// list keyed by JSON pointer so each line points to a distinct problem.
func flattenSchemaErrors(file string, err error) []ValidationError {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []ValidationError{{File: file, Message: err.Error()}}
	}
	var out []ValidationError
	var walk func(*jsonschema.ValidationError)
	walk = func(v *jsonschema.ValidationError) {
		if len(v.Causes) == 0 {
			out = append(out, ValidationError{
				File:    file,
				Path:    jsonPointerToDotPath(v.InstanceLocation),
				Message: v.Message,
			})
			return
		}
		for _, c := range v.Causes {
			walk(c)
		}
	}
	walk(ve)

	// Stable ordering: by path, then message.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// jsonPointerToDotPath converts JSON-pointer style ("/sections/0/endpoints/1/method")
// into a more readable dotted path (".sections[0].endpoints[1].method").
func jsonPointerToDotPath(p string) string {
	if p == "" {
		return "(root)"
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	var b strings.Builder
	for _, part := range parts {
		if isNumeric(part) {
			b.WriteString("[")
			b.WriteString(part)
			b.WriteString("]")
		} else {
			b.WriteString(".")
			b.WriteString(part)
		}
	}
	if b.Len() == 0 {
		return "(root)"
	}
	return b.String()
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// decodeYAMLToJSON loads a YAML file into a JSON-compatible value for schema
// validation of the raw document (used by ValidateFile to enforce
// additionalProperties / catch typos before the struct unmarshal hides them).
func decodeYAMLToJSON(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	// yaml.v3 gives map[interface{}]interface{} for maps — convert to map[string]interface{}
	// so jsonschema can walk it.
	return normalizeYAMLValue(m), nil
}

func normalizeYAMLValue(v any) any {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[fmt.Sprintf("%v", k)] = normalizeYAMLValue(vv)
		}
		return m
	case map[string]interface{}:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = normalizeYAMLValue(vv)
		}
		return m
	case []interface{}:
		for i, vv := range x {
			x[i] = normalizeYAMLValue(vv)
		}
		return x
	}
	return v
}
