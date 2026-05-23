package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadSpecFromPath loads spec from a file or directory.
// If path is a directory, loads index.yaml + merges all .yaml files recursively.
// If path is a file named index.yaml, uses directory mode on parent (auto-include siblings).
// Otherwise loads the single file directly (backward compatible).
func loadSpecFromPath(path string) (*APISpec, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	if info.IsDir() {
		return loadDirSpec(path)
	}

	// If the file is index.yaml, auto-include sibling files from parent directory
	if strings.EqualFold(filepath.Base(path), "index.yaml") {
		return loadDirSpec(filepath.Dir(path))
	}

	return loadFileSpec(path)
}

// loadFileSpec loads a single YAML file into APISpec. When the file is
// detected as an OpenAPI document, it is projected through the OpenAPI
// importer instead.
func loadFileSpec(path string) (*APISpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file %s: %w", path, err)
	}

	if isOpenAPIDocument(data) {
		return LoadOpenAPISpec(path)
	}

	var spec APISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML %s: %w", path, err)
	}

	return &spec, nil
}

// isOpenAPIDocument heuristically detects an OpenAPI 3.x document.
//
// Look only at the first ~4 KB and only at lines where `openapi:` (YAML) or
// `"openapi":` (JSON) appears at column 0 — i.e. as a top-level key, not as
// part of a description or a nested field. The previous implementation
// matched the literal substring anywhere on a line that started with the
// substring, which still false-positived on documents whose top-level
// description happened to start a paragraph with the word "openapi:".
//
// We also require the value side to look like a version string starting
// with `3.` (the only OpenAPI major version we project; OpenAPI 2.0 / Swagger
// uses `swagger: "2.0"`). This rules out YAML keys like `openapi:` being
// used as a literal field name in a docs-generator spec.
// openAPIJSONRe matches the `"openapi": "3.x..."` field at the start of a
// JSON document (with optional leading whitespace/object-open). This catches
// minified JSON where the per-line check would miss the field — pretty-
// printed JSON is also covered because the regex is anchored to the head
// of the (already trimmed) buffer.
var openAPIJSONRe = regexp.MustCompile(`^[\s\x{feff}]*\{[\s]*"openapi"\s*:\s*"3\.`)

func isOpenAPIDocument(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	// JSON form: check the head-of-document regex first. Minified one-liner
	// specs like `{"openapi":"3.0.0",...}` only match here, not in the
	// per-line YAML pass.
	if openAPIJSONRe.Match(head) {
		return true
	}
	for _, line := range splitLines(head) {
		// YAML: `openapi: 3.0.3` or `openapi: "3.1.0"` at column 0.
		if rest, ok := trimKeyPrefix(line, "openapi:"); ok {
			rest = trimQuotesAndSpaces(rest)
			if strings.HasPrefix(rest, "3.") {
				return true
			}
		}
	}
	return false
}

// splitLines returns the byte-slice lines of data without allocating extra
// copies of the underlying bytes.
func splitLines(data []byte) []string {
	// Fine to convert to string here — the OpenAPI heuristic runs once per
	// load, not per request.
	return strings.Split(string(data), "\n")
}

// trimKeyPrefix returns the substring after `prefix` if line starts with it.
func trimKeyPrefix(line, prefix string) (string, bool) {
	if strings.HasPrefix(line, prefix) {
		return line[len(prefix):], true
	}
	return "", false
}

// trimQuotesAndSpaces strips surrounding whitespace and a leading quote so
// the caller can match the start of the version literal regardless of
// whether it was written as `3.0.3` or `"3.0.3"`.
func trimQuotesAndSpaces(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, `"'`)
	return s
}

// loadDirSpec loads index.yaml from directory and merges all other .yaml files.
func loadDirSpec(dir string) (*APISpec, error) {
	indexPath := filepath.Join(dir, "index.yaml")

	base, err := loadFileSpec(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load index.yaml in %s: %w", dir, err)
	}

	// Collect all .yaml files except index.yaml, sorted for deterministic order.
	//
	// Subdirectories that contain their OWN index.yaml are project boundaries
	// (see discoverProjects) — skip them entirely so the root/default project
	// does not absorb every subproject's sections and have its info clobbered.
	// Plain overlay subdirs (sections/, guides/, …) have no index.yaml and are
	// merged as before.
	var yamlFiles []string
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Surface traversal failures rather than silently rendering a
			// partial spec (e.g. an unreadable overlay directory).
			return err
		}
		if d.IsDir() {
			if path != dir {
				if _, statErr := os.Stat(filepath.Join(path, "index.yaml")); statErr == nil {
					return filepath.SkipDir // separate project — not part of this one
				}
			}
			return nil
		}
		if path == indexPath {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".yaml") || strings.HasSuffix(strings.ToLower(path), ".yml") {
			yamlFiles = append(yamlFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan spec directory %s: %w", dir, walkErr)
	}
	sort.Strings(yamlFiles)

	// Merge each file into base
	for _, f := range yamlFiles {
		overlay, err := loadFileSpec(f)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", f, err)
		}
		mergeSpec(base, overlay)
	}

	return base, nil
}

// mergeSpec merges overlay into base using generic, reflection-driven rules:
//   - slice fields whose element type carries an identifying field are
//     merged BY KEY (base entries keep their position; overlay entries
//     with a matching key recurse into the existing entry; non-matching
//     overlay entries are appended)
//   - other slice fields are APPENDED (additive)
//   - nested struct fields RECURSE (field-by-field merge)
//   - scalar fields are OVERRIDDEN when overlay is non-zero
//
// Merge-by-key prevents the obvious footgun where two overlay files each
// declare a section with the same id and the renderer ends up emitting two
// sections (anchor collisions, duplicated nav entries). Adding a new
// id-bearing struct anywhere in APISpec automatically opts into key merge
// — no extra wiring needed.
func mergeSpec(base, overlay *APISpec) {
	mergeStruct(reflect.ValueOf(base).Elem(), reflect.ValueOf(overlay).Elem())
}

// mergeStruct walks each field of two struct values of the same type and
// applies the merge rules above. base must be addressable.
func mergeStruct(base, overlay reflect.Value) {
	for i := 0; i < base.NumField(); i++ {
		mergeField(base.Field(i), overlay.Field(i))
	}
}

func mergeField(base, overlay reflect.Value) {
	switch base.Kind() {
	case reflect.Slice:
		if overlay.Len() == 0 {
			return
		}
		if keyField, ok := sliceKeyField(base.Type().Elem()); ok {
			base.Set(mergeSliceByKey(base, overlay, keyField))
			return
		}
		base.Set(reflect.AppendSlice(base, overlay))
	case reflect.Struct:
		mergeStruct(base, overlay)
	case reflect.Pointer:
		if !overlay.IsNil() {
			base.Set(overlay)
		}
	default:
		// scalar (string, int, bool, float, …): overlay wins when non-zero.
		if !overlay.IsZero() {
			base.Set(overlay)
		}
	}
}

// sliceKeyField returns the name of the struct field that identifies an
// entry for merge-by-key purposes, or ("", false) if the element type is
// not keyable. Only the canonical "ID" field is honoured — using "Name"
// as a key is unsafe because Endpoint.Name is human-readable, not a
// stable identifier; two endpoints in the same section might legitimately
// share a name while differing by method/path. Permissions use Name as
// their identity but the duplicate-permission check lives in the linter,
// so missing the merge here only means the lint warning fires instead of
// silent collapse.
func sliceKeyField(elem reflect.Type) (string, bool) {
	if elem.Kind() != reflect.Struct {
		return "", false
	}
	if f, ok := elem.FieldByName("ID"); ok && f.Type.Kind() == reflect.String {
		return "ID", true
	}
	return "", false
}

// mergeSliceByKey merges overlay entries into base by the named string
// field. Base order is preserved. Overlay entries whose key matches a base
// entry recurse into that entry (so a section's endpoints accumulate
// across files); non-matching entries are appended at the end. Entries
// with an empty key fall through to plain append — every file is free to
// add unkeyed rows but they won't collide.
func mergeSliceByKey(base, overlay reflect.Value, keyField string) reflect.Value {
	// Index base entries by their key value so the overlay pass is O(n).
	index := make(map[string]int, base.Len())
	for i := 0; i < base.Len(); i++ {
		k := base.Index(i).FieldByName(keyField).String()
		if k != "" {
			index[k] = i
		}
	}

	for j := 0; j < overlay.Len(); j++ {
		ov := overlay.Index(j)
		k := ov.FieldByName(keyField).String()
		if k == "" {
			base = reflect.Append(base, ov)
			continue
		}
		if i, ok := index[k]; ok {
			// Recurse into the existing base entry so nested slices
			// (e.g. SectionInfo.Endpoints) accumulate too.
			mergeStruct(base.Index(i), ov)
			continue
		}
		index[k] = base.Len()
		base = reflect.Append(base, ov)
	}
	return base
}

// discoverProjects scans a root directory for sub-directories containing index.yaml.
// Returns map of project name → APISpec. The root itself is the default project (key "").
func discoverProjects(root string) (map[string]*APISpec, error) {
	projects := make(map[string]*APISpec)

	// Load default project from root
	defaultSpec, err := loadDirSpec(root)
	if err != nil {
		return nil, fmt.Errorf("failed to load default project: %w", err)
	}
	projects[""] = defaultSpec

	// Scan sub-directories for other projects
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(root, entry.Name())
		indexPath := filepath.Join(subDir, "index.yaml")

		// Only treat as project if it has index.yaml
		if _, err := os.Stat(indexPath); err != nil {
			continue
		}

		spec, err := loadDirSpec(subDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load project %s: %w", entry.Name(), err)
		}
		projects[entry.Name()] = spec
	}

	return projects, nil
}
