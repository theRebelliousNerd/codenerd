package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// ManifestPin is one dependency version a workspace manifest pins: a go.mod
// require or a package.json dependency.
type ManifestPin struct {
	Ecosystem Ecosystem
	// Module is the module path (Go) or package name (npm).
	Module string
	// Version is the pinned version as the manifest spells it.
	Version string
	// Manifest is the workspace-relative manifest path.
	Manifest string
}

// ManifestPins reads every go.mod and package.json in the workspace (root
// first, then nested modules) and returns what each pins, sorted. A manifest
// that cannot be read or parsed is an error naming it; the pins of the others
// are still returned.
func ManifestPins(workspace string) ([]ManifestPin, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, nil
	}
	var pins []ManifestPin
	var failures []string
	seen := map[string]bool{}
	for _, name := range []string{"go.mod", "package.json"} {
		paths := append([]string{filepath.Join(workspace, name)}, findManifestFiles(workspace, []string{name}, maxManifestDepth)...)
		for _, path := range paths {
			if seen[path] {
				continue
			}
			seen[path] = true
			data, err := os.ReadFile(path)
			if err != nil {
				if !os.IsNotExist(err) {
					failures = append(failures, fmt.Sprintf("%s: %v", path, err))
				}
				continue
			}
			rel, relErr := filepath.Rel(workspace, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			got, err := parseManifestPins(name, rel, data)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", rel, err))
				continue
			}
			pins = append(pins, got...)
		}
	}
	sort.Slice(pins, func(i, j int) bool {
		a, b := pins[i], pins[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Manifest != b.Manifest {
			return a.Manifest < b.Manifest
		}
		return a.Version < b.Version
	})
	if len(failures) > 0 {
		return pins, fmt.Errorf("manifests not read: %s", strings.Join(failures, "; "))
	}
	return pins, nil
}

func parseManifestPins(name, rel string, data []byte) ([]ManifestPin, error) {
	var pins []ManifestPin
	switch name {
	case "go.mod":
		f, err := modfile.ParseLax(rel, data, nil)
		if err != nil {
			return nil, err
		}
		for _, req := range f.Require {
			pins = append(pins, ManifestPin{Ecosystem: EcosystemGo, Module: req.Mod.Path, Version: req.Mod.Version, Manifest: rel})
		}
	case "package.json":
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return nil, err
		}
		for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
			for module, version := range deps {
				pins = append(pins, ManifestPin{Ecosystem: EcosystemNPM, Module: module, Version: version, Manifest: rel})
			}
		}
	}
	return pins, nil
}
