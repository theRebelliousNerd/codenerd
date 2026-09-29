package orient

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// discover walks root for the known agent-CLI layouts and, when home is set,
// Claude Code project memory. root may itself be named .nerd; a .nerd
// directory inside it is not a corpus. Git failure leaves Tracked false and
// is not an error: a tree that is not a repository is still a corpus.
func discover(root, home string) ([]Source, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(abs)
	tracked := gitTracked(root)
	rels, walkErr := collect(root)

	dirs := skillDirs(rels)
	var sources []Source
	var readErrs []error
	if walkErr != nil {
		readErrs = append(readErrs, walkErr)
	}
	seen := map[string]struct{}{}
	parents := map[string]Source{}
	type pending struct {
		rel string
		dir string
	}
	var refs []pending

	for _, rel := range rels {
		// settings.json is the Gemini CLI's own instructions, even when a
		// SKILL.md directory would otherwise claim every file under it.
		// Jules journals are the same kind of exception: a CLAUDE.md in
		// there is a journal, not a Claude instruction file.
		if !underJules(rel) && !isGeminiSettings(rel) {
			if dir, ok := owningSkillDir(rel, dirs); ok {
				refs = append(refs, pending{rel, dir})
				continue
			}
		}
		tool, kind, ok := classify(rel)
		if !ok {
			continue
		}
		parsed, err := loadParsed(root, rel, tool, kind, tracked)
		if err != nil {
			readErrs = append(readErrs, err)
			continue
		}
		for _, src := range parsed {
			src.ID = takeID(src.ID, seen)
			sources = append(sources, src)
			if path.Base(rel) == "SKILL.md" && src.Kind == "skill" {
				parents[path.Dir(rel)] = src
			}
		}
	}

	for _, ref := range refs {
		parent, ok := parents[ref.dir]
		var src Source
		var err error
		if ok {
			src, err = loadReference(root, ref.rel, parent, tracked)
		} else {
			src, err = loadOne(root, ref.rel, toolForSkillDir(ref.dir), "skill", tracked)
		}
		if err != nil {
			readErrs = append(readErrs, err)
			continue
		}
		src.ID = takeID(src.ID, seen)
		sources = append(sources, src)
	}

	mem, err := claudeMemory(root, home)
	if err != nil {
		readErrs = append(readErrs, err)
	}
	for _, src := range mem {
		src.ID = takeID(src.ID, seen)
		sources = append(sources, src)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	return sources, errors.Join(readErrs...)
}

func collect(root string) ([]string, error) {
	var rels []string
	var errs []error
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, walkErr)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			// The walk starts at root even when that directory is named .nerd.
			// A nested .nerd is this product's own state, not an agent corpus.
			if d.Name() == ".nerd" && filepath.Clean(p) != root {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		rel = filepath.ToSlash(rel)
		if keepCandidate(rel) {
			rels = append(rels, rel)
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	sort.Strings(rels)
	return rels, errors.Join(errs...)
}

func keepCandidate(rel string) bool {
	base := path.Base(rel)
	if base == ".roomodes" {
		return true
	}
	switch strings.ToLower(path.Ext(base)) {
	case ".md", ".mdc", ".txt", ".toml", ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func gitTracked(root string) map[string]bool {
	cmd := exec.Command("git", "-C", root, "-c", "core.quotepath=off", "ls-files", "-z")
	cmd.Env = withEnv(os.Environ(), map[string]string{"GIT_OPTIONAL_LOCKS": "0"})
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		set[strings.ReplaceAll(string(p), "\\", "/")] = true
	}
	return set
}

func skillDirs(rels []string) []string {
	var dirs []string
	for _, rel := range rels {
		if path.Base(rel) != "SKILL.md" || !isKnownSkill(rel) {
			continue
		}
		dirs = append(dirs, path.Dir(rel))
	}
	sort.Slice(dirs, func(i, j int) bool {
		if len(dirs[i]) != len(dirs[j]) {
			return len(dirs[i]) > len(dirs[j])
		}
		return dirs[i] < dirs[j]
	})
	return dirs
}

func isKnownSkill(rel string) bool {
	parts := strings.Split(rel, "/")
	if hasSegAfter(parts, ".claude", "skills") || hasSegAfter(parts, ".codex", "skills") ||
		hasSegAfter(parts, ".agents", "skills") || hasSegAfter(parts, ".gemini", "skills") {
		return true
	}
	return hasSeg(parts, ".grok") && hasSeg(parts, "skills")
}

func owningSkillDir(rel string, dirs []string) (string, bool) {
	for _, dir := range dirs {
		if rel == dir+"/SKILL.md" {
			return "", false
		}
		if strings.HasPrefix(rel, dir+"/") {
			return dir, true
		}
	}
	return "", false
}

func toolForSkillDir(dir string) string {
	return toolOfPath(dir)
}

func classify(rel string) (tool, kind string, ok bool) {
	parts := strings.Split(rel, "/")
	base := parts[len(parts)-1]
	ext := strings.ToLower(path.Ext(base))
	if hasSeg(parts, ".jules") {
		return "jules", "journal", true
	}
	if isGeminiSettings(rel) {
		return "gemini", "instructions", true
	}
	if hasSeg(parts, ".claude") {
		if tool, kind, ok = classifyClaude(parts, base, ext); ok {
			return tool, kind, true
		}
	}
	if hasSeg(parts, ".codex") {
		if tool, kind, ok = classifyCodex(parts, base, ext); ok {
			return tool, kind, true
		}
	}
	if hasSeg(parts, ".agents") {
		if tool, kind, ok = classifyAgents(parts, base, ext); ok {
			return tool, kind, true
		}
	}
	if hasSeg(parts, ".gemini") {
		if tool, kind, ok = classifyGemini(base, ext); ok {
			return tool, kind, true
		}
	}
	if hasSeg(parts, ".cursor") {
		if tool, kind, ok = classifyCursor(parts, ext); ok {
			return tool, kind, true
		}
	}
	if hasSeg(parts, ".grok") {
		if tool, kind, ok = classifyGrok(parts, base, ext); ok {
			return tool, kind, true
		}
	}
	if base == ".roomodes" {
		return "roo", "mode", true
	}
	switch base {
	case "CLAUDE.md":
		return "claude", "instructions", true
	case "AGENTS.md":
		return "agents", "instructions", true
	case "GEMINI.md":
		return "gemini", "instructions", true
	}
	if base == "copilot-instructions.md" && len(parts) >= 2 && parts[len(parts)-2] == ".github" {
		return "copilot", "instructions", true
	}
	return "", "", false
}

func classifyClaude(parts []string, base, ext string) (string, string, bool) {
	switch {
	case hasSegAfter(parts, ".claude", "skills") && base == "SKILL.md":
		return "claude", "skill", true
	case hasSegAfter(parts, ".claude", "agents") && (ext == ".md" || ext == ".mdc"):
		return "claude", "subagent", true
	case hasSegAfter(parts, ".claude", "agent-memory") && (ext == ".md" || ext == ".mdc"):
		return "claude", "memory", true
	case hasSegAfter(parts, ".claude", "commands") && (ext == ".md" || ext == ".mdc"):
		return "claude", "command", true
	default:
		return "", "", false
	}
}

func classifyCodex(parts []string, base, ext string) (string, string, bool) {
	switch {
	case hasSegAfter(parts, ".codex", "skills") && base == "SKILL.md":
		return "codex", "skill", true
	case hasSegAfter(parts, ".codex", "agents") && ext == ".toml":
		return "codex", "subagent", true
	default:
		return "", "", false
	}
}

func classifyAgents(parts []string, base, ext string) (string, string, bool) {
	switch {
	case hasSegAfter(parts, ".agents", "skills") && base == "SKILL.md":
		return "agents", "skill", true
	case hasSegAfter(parts, ".agents", "rules") && (ext == ".md" || ext == ".mdc"):
		return "agents", "rule", true
	default:
		return "", "", false
	}
}

func classifyGemini(base, ext string) (string, string, bool) {
	if base == "SKILL.md" || ext == ".md" || ext == ".mdc" || ext == ".txt" || ext == ".toml" {
		return "gemini", "skill", true
	}
	return "", "", false
}

func classifyCursor(parts []string, ext string) (string, string, bool) {
	if hasSegAfter(parts, ".cursor", "rules") && (ext == ".md" || ext == ".mdc") {
		return "cursor", "rule", true
	}
	return "", "", false
}

func classifyGrok(parts []string, base, ext string) (string, string, bool) {
	doc := ext == ".md" || ext == ".mdc" || ext == ".txt" || ext == ".toml"
	switch {
	case hasSeg(parts, "skills") && base == "SKILL.md":
		return "grok", "skill", true
	case hasSeg(parts, "agents") && doc:
		return "grok", "subagent", true
	case hasSeg(parts, "rules") && (ext == ".md" || ext == ".mdc"):
		return "grok", "rule", true
	case hasSeg(parts, "commands") && doc:
		return "grok", "command", true
	case hasSeg(parts, "memory") && doc:
		return "grok", "memory", true
	case doc:
		return "grok", "instructions", true
	default:
		return "", "", false
	}
}

func underJules(rel string) bool {
	return hasSeg(strings.Split(rel, "/"), ".jules")
}

func isGeminiSettings(rel string) bool {
	return rel == ".gemini/settings.json" || strings.HasSuffix(rel, "/.gemini/settings.json")
}

func hasSeg(parts []string, want string) bool {
	for _, p := range parts {
		if p == want {
			return true
		}
	}
	return false
}

func hasSegAfter(parts []string, marker, want string) bool {
	seen := false
	for _, p := range parts {
		if seen && p == want {
			return true
		}
		if p == marker {
			seen = true
		}
	}
	return false
}

func toolOfPath(rel string) string {
	parts := strings.Split(rel, "/")
	switch {
	case hasSeg(parts, ".claude"):
		return "claude"
	case hasSeg(parts, ".codex"):
		return "codex"
	case hasSeg(parts, ".agents"):
		return "agents"
	case hasSeg(parts, ".gemini"):
		return "gemini"
	case hasSeg(parts, ".grok"):
		return "grok"
	default:
		return "other"
	}
}

func takeID(id string, seen map[string]struct{}) string {
	if _, ok := seen[id]; !ok {
		seen[id] = struct{}{}
		return id
	}
	for n := 2; ; n++ {
		next := fmt.Sprintf("%s#%d", id, n)
		if _, ok := seen[next]; !ok {
			seen[next] = struct{}{}
			return next
		}
	}
}

func loadParsed(root, rel, tool, kind string, tracked map[string]bool) ([]Source, error) {
	text, err := readRel(root, rel)
	if err != nil {
		return nil, err
	}
	tr := tracked[rel]
	var parsed []Source
	switch {
	case kind == "mode" && path.Base(rel) == ".roomodes":
		parsed = parseRoomodes(rel, text, tr)
	case strings.EqualFold(path.Ext(rel), ".toml"):
		parsed = []Source{parseTOMLSource(rel, tool, kind, text, tr)}
	case isGeminiSettings(rel):
		parsed = []Source{parseSettings(rel, text, tr)}
	default:
		parsed = []Source{parseMarkdown(rel, tool, kind, text, tr)}
	}
	for i := range parsed {
		finalize(&parsed[i])
	}
	return parsed, nil
}

func loadOne(root, rel, tool, kind string, tracked map[string]bool) (Source, error) {
	parsed, err := loadParsed(root, rel, tool, kind, tracked)
	if err != nil {
		return Source{}, err
	}
	if len(parsed) == 0 {
		return Source{}, fmt.Errorf("read %s: no source", rel)
	}
	return parsed[0], nil
}

func loadReference(root, rel string, parent Source, tracked map[string]bool) (Source, error) {
	text, err := readRel(root, rel)
	if err != nil {
		return Source{}, err
	}
	src := parseMarkdown(rel, parent.Tool, "skill", text, tracked[rel])
	// The reference belongs to the skill. Its own description stays; the
	// parent's long description does not, or every sibling would look like
	// the same document.
	src.Name = parent.Name
	src.Tags = append([]string(nil), parent.Tags...)
	finalize(&src)
	return src, nil
}

func readRel(root, rel string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return normalizeText(string(raw)), nil
}

func claudeProjectSlug(root string) string {
	var b strings.Builder
	b.Grow(len(root))
	for _, r := range root {
		switch r {
		case ':', '\\', '/':
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func claudeMemory(root, home string) ([]Source, error) {
	if strings.TrimSpace(home) == "" {
		return nil, nil
	}
	dir := filepath.Join(home, ".claude", "projects", claudeProjectSlug(root), "memory")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Source
	var errs []error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		abs := filepath.Join(dir, e.Name())
		id := filepath.ToSlash(abs)
		raw, err := os.ReadFile(abs)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", id, err))
			continue
		}
		src := parseMarkdown(id, "claude", "memory", normalizeText(string(raw)), false)
		src.ID = id
		src.Path = id
		src.Tracked = false
		finalize(&src)
		out = append(out, src)
	}
	return out, errors.Join(errs...)
}
