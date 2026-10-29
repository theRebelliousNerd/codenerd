package init

import (
	"regexp"
	"strings"
)

// Direct dependencies declared by Python and Rust manifests, filtered to the
// technology vocabulary (technology_vocabulary.go). Init read only their lock
// files, so a Python or Rust project without a committed lock declared no
// dependency at all and the catalog created no agent for its frameworks.
// The module carries no TOML library (internal/orient/ecosystem_toml.go hand
// parses too): these read the few shapes dependency tables take.

var pythonNameRun = regexp.MustCompile(`[-_.]+`)

// normalizePythonName is a distribution name as PEP 503 compares it.
func normalizePythonName(name string) string {
	return pythonNameRun.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
}

// pythonRequirementName is the distribution a requirement line names: the
// text before any version, marker, extra or URL.
func pythonRequirementName(req string) string {
	req = strings.TrimSpace(req)
	if i := strings.IndexAny(req, "<>=!~;[ @("); i >= 0 {
		req = req[:i]
	}
	return normalizePythonName(req)
}

// pythonDeps maps requirement names through the vocabulary, first wins.
func pythonDeps(names []string) []DependencyInfo {
	var deps []DependencyInfo
	seen := map[string]bool{}
	for _, raw := range names {
		name := pythonRequirementName(raw)
		key, ok := technologyFor(EcosystemPython, name)
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		deps = append(deps, DependencyInfo{Name: key, Type: "direct"})
	}
	return deps
}

// parseRequirementsTxt reads requirements.txt: one requirement per line,
// comments and pip options skipped.
func parseRequirementsTxt(content string) []DependencyInfo {
	var names []string
	for line := range strings.SplitSeq(content, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		names = append(names, line)
	}
	return pythonDeps(names)
}

// tomlTable is the table a header line opens ("[tool.poetry.dependencies]"
// -> "tool.poetry.dependencies"), or "" when the line is not a header.
func tomlTable(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return ""
	}
	return strings.TrimSpace(strings.Trim(line, "[]"))
}

// tomlKey is the bare key of a "key = value" line, unquoted.
func tomlKey(line string) string {
	i := strings.Index(line, "=")
	if i <= 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(line[:i]), `"'`)
}

var tomlQuoted = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)

// parsePyprojectDependencies reads pyproject.toml: PEP 621's
// [project] dependencies array, and Poetry's dependency tables (whose keys
// are the distributions; "python" is the interpreter, not a dependency).
func parsePyprojectDependencies(content string) []DependencyInfo {
	var names []string
	table := ""
	inArray := false
	for raw := range strings.SplitSeq(content, "\n") {
		line := raw
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if inArray {
			for _, m := range tomlQuoted.FindAllStringSubmatch(line, -1) {
				names = append(names, m[1]+m[2])
			}
			if strings.Contains(line, "]") {
				inArray = false
			}
			continue
		}
		if t := tomlTable(line); t != "" {
			table = t
			continue
		}
		switch {
		case table == "project" && tomlKey(line) == "dependencies":
			rest := line[strings.Index(line, "=")+1:]
			for _, m := range tomlQuoted.FindAllStringSubmatch(rest, -1) {
				names = append(names, m[1]+m[2])
			}
			inArray = strings.Contains(rest, "[") && !strings.Contains(rest, "]")
		case table == "tool.poetry.dependencies" ||
			(strings.HasPrefix(table, "tool.poetry.group.") && strings.HasSuffix(table, ".dependencies")):
			if key := tomlKey(line); key != "" && key != "python" {
				names = append(names, key)
			}
		}
	}
	return pythonDeps(names)
}

// parseCargoTomlDependencies reads Cargo.toml's dependency tables
// ([dependencies], [dev-dependencies], [build-dependencies], their
// [target.*] forms, and [dependencies.<crate>] headers).
func parseCargoTomlDependencies(content string) []DependencyInfo {
	var deps []DependencyInfo
	seen := map[string]bool{}
	add := func(crate string) {
		key, ok := technologyFor(EcosystemRust, strings.TrimSpace(crate))
		if !ok || seen[key] {
			return
		}
		seen[key] = true
		deps = append(deps, DependencyInfo{Name: key, Type: "direct"})
	}
	isDepTable := func(t string) bool {
		last := t
		if i := strings.LastIndex(t, "."); i >= 0 {
			last = t[i+1:]
		}
		return last == "dependencies" || last == "dev-dependencies" || last == "build-dependencies"
	}
	table := ""
	for raw := range strings.SplitSeq(content, "\n") {
		line := raw
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if t := tomlTable(line); t != "" {
			table = t
			// [dependencies.tokio]
			for _, prefix := range []string{"dependencies.", "dev-dependencies.", "build-dependencies."} {
				if crate, ok := strings.CutPrefix(t, prefix); ok {
					add(strings.Trim(crate, `"'`))
				}
			}
			continue
		}
		if isDepTable(table) {
			if key := tomlKey(line); key != "" {
				add(key)
			}
		}
	}
	return deps
}
