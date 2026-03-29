package docs

import (
	"fmt"
	"os"
	"path/filepath"
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

// loadFileSpec loads a single YAML file into APISpec.
func loadFileSpec(path string) (*APISpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file %s: %w", path, err)
	}

	var spec APISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML %s: %w", path, err)
	}

	return &spec, nil
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

// mergeSpec merges overlay into base.
// Array fields are appended, scalar/struct fields are overridden.
func mergeSpec(base, overlay *APISpec) {
	// Override scalars/structs only if overlay has content
	if overlay.Info.Title != "" {
		base.Info = overlay.Info
	}
	if len(overlay.Authentication.Methods) > 0 {
		base.Authentication = overlay.Authentication
	}
	if len(overlay.FlowOverview.Methods) > 0 {
		base.FlowOverview = overlay.FlowOverview
	}
	if overlay.APITesterDefaults.Methods != nil || overlay.APITesterDefaults.AuthModes != nil {
		base.APITesterDefaults = overlay.APITesterDefaults
	}

	// Append arrays
	base.Sections = append(base.Sections, overlay.Sections...)
	base.Guides = append(base.Guides, overlay.Guides...)
	base.Permissions = append(base.Permissions, overlay.Permissions...)
	base.Constraints = append(base.Constraints, overlay.Constraints...)
	base.FlowDiagramNodes = append(base.FlowDiagramNodes, overlay.FlowDiagramNodes...)
	base.FlowDiagramEdges = append(base.FlowDiagramEdges, overlay.FlowDiagramEdges...)

	// Append nested arrays from Info
	if len(overlay.Info.OverviewCards) > 0 {
		base.Info.OverviewCards = append(base.Info.OverviewCards, overlay.Info.OverviewCards...)
	}
	if len(overlay.Info.BaseURLs) > 0 {
		base.Info.BaseURLs = append(base.Info.BaseURLs, overlay.Info.BaseURLs...)
	}

	// Append nested arrays from FlowOverview
	if len(overlay.FlowOverview.Methods) > 0 {
		base.FlowOverview.Methods = append(base.FlowOverview.Methods, overlay.FlowOverview.Methods...)
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
