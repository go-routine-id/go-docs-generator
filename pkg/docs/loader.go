package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// isOpenAPIDocument heuristically detects an OpenAPI 3.x document by looking
// for the `openapi:` (YAML) or `"openapi":` (JSON) top-level key.
func isOpenAPIDocument(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	return bytesContainsLine(head, "openapi:") || bytesContainsLine(head, `"openapi"`)
}

func bytesContainsLine(data []byte, needle string) bool {
	n := []byte(needle)
	for i := 0; i+len(n) <= len(data); i++ {
		if (i == 0 || data[i-1] == '\n') && string(data[i:i+len(n)]) == needle {
			return true
		}
	}
	return false
}

// loadDirSpec loads index.yaml from directory and merges all other .yaml files.
func loadDirSpec(dir string) (*APISpec, error) {
	indexPath := filepath.Join(dir, "index.yaml")

	base, err := loadFileSpec(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load index.yaml in %s: %w", dir, err)
	}

	// Collect all .yaml files except index.yaml, sorted for deterministic order
	var yamlFiles []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
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
//   - slice fields are APPENDED (additive — every overlay file contributes)
//   - nested struct fields RECURSE (field-by-field merge)
//   - scalar fields are OVERRIDDEN when overlay is non-zero
//
// This contract means new top-level or nested fields in APISpec do NOT require
// updating this function — adding a field to the struct is enough.
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
		if overlay.Len() > 0 {
			base.Set(reflect.AppendSlice(base, overlay))
		}
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
