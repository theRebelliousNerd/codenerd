package session

import (
	"fmt"
	"path"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
)

// Module guidance. Every nerd.md below the workspace root belongs to its
// directory: its frontmatter is asserted into the kernel with the root's, and
// its body is that directory's contributor guidance (each directory's
// agents.md, generated into nerd.md by projectdoc.AgentDocTwins). A turn
// whose target lies under such a directory gets that module's body, then the
// bodies of the module directories above it, nearest first and never the
// root's (which is the project instructions already), the way Claude Code
// reads a subdirectory's CLAUDE.md.
//
// Outside a working loop the guidance goes on the system prompt with the
// project instructions. Inside one it is part of the focus view, so it is
// sent when the focus enters a module and replaced when the focus moves to
// another, not resent every round.

// Where module guidance is rendered (moduleGuidance's where).
const (
	guidanceOnSystemPrompt = "system prompt"
	guidanceInFocusView    = "focus view"
)

// moduleGuidanceHeading opens one module's guidance; the directory follows.
const moduleGuidanceHeading = "## Module guidance: "

// SetProjectModuleDocs attaches the workspace's module nerd.md documents
// (projectdoc.LoadAll without the root document).
func (e *Executor) SetProjectModuleDocs(docs []*projectdoc.Document) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.projectModules = docs
}

// moduleGuidance renders the guidance of the modules enclosing target,
// nearest first. budget is in bytes: 0 is unbounded, negative leaves no room.
// A module whose body does not fit is not cut: it and the farther ones are
// named in one line instead, with their size, so the model knows guidance
// exists and where to read it. where is guidanceOnSystemPrompt or
// guidanceInFocusView; only a system-prompt render is logged as a send.
func (e *Executor) moduleGuidance(target string, budget int, where string) string {
	e.mu.RLock()
	docs := e.projectModules
	e.mu.RUnlock()
	if len(docs) == 0 {
		return ""
	}
	// A target outside the workspace has no module.
	rel := strings.TrimSpace(target)
	if ws := e.workspaceForVerification(); ws != "" && rel != "" {
		var err error
		if rel, err = workspaceRelative(ws, rel); err != nil {
			return ""
		}
	}
	chain := moduleChain(docs, rel)
	if len(chain) == 0 {
		return ""
	}
	var parts, left []string
	leftBytes, used := 0, 0
	for _, doc := range chain {
		body := strings.TrimSpace(doc.Body)
		if body == "" {
			continue
		}
		section := moduleGuidanceHeading + path.Dir(doc.Path) + "\n\n" + body
		if len(left) > 0 || budget < 0 || (budget > 0 && used+len(section) > budget) {
			left = append(left, doc.Path)
			leftBytes += len(section)
			continue
		}
		parts = append(parts, section)
		used += len(section)
		if where == guidanceOnSystemPrompt {
			// The focus view is rendered every round and appended only when
			// it changed; the ledger logs that append (updateLedger), so a
			// render is not reported as a send here.
			logging.Session("Injected module guidance %s into the system prompt for %s (%d chars)", doc.Path, target, len(section))
		}
	}
	if len(left) > 0 {
		parts = append(parts, fmt.Sprintf("[module guidance not shown, %d chars over the budget: %s; read them with read_file when the task needs them]",
			leftBytes, strings.Join(left, ", ")))
		if where == guidanceOnSystemPrompt {
			logging.Session("Module guidance for %s left out of the system prompt for the budget: %s (%d chars)", target, strings.Join(left, ", "), leftBytes)
		}
	}
	return strings.Join(parts, "\n\n")
}

// moduleChain is the module documents whose directory encloses target,
// nearest first, one per directory. The root nerd.md is not a module.
func moduleChain(docs []*projectdoc.Document, target string) []*projectdoc.Document {
	target = path.Clean(strings.ReplaceAll(strings.TrimSpace(target), "\\", "/"))
	if target == "." || target == "" || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
		return nil
	}
	byDir := make(map[string]*projectdoc.Document, len(docs))
	for _, d := range docs {
		if d == nil {
			continue
		}
		dir := path.Dir(path.Clean(d.Path))
		if dir == "." {
			continue
		}
		if _, dup := byDir[dir]; !dup {
			byDir[dir] = d
		}
	}
	var chain []*projectdoc.Document
	// The target itself may be a directory; then walk up from it.
	for dir := target; dir != "." && dir != "/"; dir = path.Dir(dir) {
		if d, ok := byDir[dir]; ok {
			chain = append(chain, d)
		}
	}
	return chain
}

// moduleGuidanceNote names the modules whose guidance a rendered view carries,
// for the ledger's append log line; "" when it carries none.
func moduleGuidanceNote(view string) string {
	var dirs []string
	for _, line := range strings.Split(view, "\n") {
		if dir, ok := strings.CutPrefix(line, moduleGuidanceHeading); ok {
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) == 0 {
		return ""
	}
	return ", module guidance: " + strings.Join(dirs, ", ")
}
