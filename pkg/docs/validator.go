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

// ValidateFile loads a YAML file (or merges a directory) and validates the
// resulting document against the embedded JSON Schema. OpenAPI inputs are
// skipped — they're already validated upstream by the kin-openapi loader.
// Returns nil on success, or a slice of ValidationError for every violation.
func ValidateFile(path string) []ValidationError {
	info, err := os.Stat(path)
	if err != nil {
		return []ValidationError{{File: path, Message: err.Error()}}
	}

	// Determine target file: if dir, use index.yaml; if file and is openapi, skip.
	target := path
	if info.IsDir() {
		target = filepath.Join(path, "index.yaml")
	}

	// Multi-file merged validation: reload specs through the standard loader,
	// then round-trip the merged APISpec through JSON to validate against schema.
	spec, err := loadSpecFromPath(target)
	if err != nil {
		return []ValidationError{{File: target, Message: err.Error()}}
	}

	// Skip OpenAPI inputs — they already passed the OpenAPI loader.
	raw, err := os.ReadFile(target)
	if err == nil && isOpenAPIDocument(raw) {
		return nil
	}

	return ValidateSpec(spec, target)
}

// ValidateSpec validates an in-memory APISpec against the embedded schema.
// file is used only for error reporting and may be empty.
func ValidateSpec(spec *APISpec, file string) []ValidationError {
	doc, err := specAsJSONValue(spec)
	if err != nil {
		return []ValidationError{{File: file, Message: "marshal spec to json: " + err.Error()}}
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("spec.schema.json", bytes.NewReader(embeddedSchemaJSON)); err != nil {
		return []ValidationError{{File: file, Message: "load embedded schema: " + err.Error()}}
	}
	schema, err := compiler.Compile("spec.schema.json")
	if err != nil {
		return []ValidationError{{File: file, Message: "compile embedded schema: " + err.Error()}}
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

// decodeYAMLToJSON loads a YAML file into a JSON-compatible value.
// Kept for future use when we validate individual overlay files.
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
