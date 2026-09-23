package session

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// briefSite is one edit site a brief names: a workspace-relative path, with
// the line when the brief gives one (path:line), else 0.
type briefSite struct {
	Path string
	Line int64
}

// briefURL is removed before paths are read: a URL's path is not a file in
// the workspace.
var briefURL = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://\S+`)

// briefPathToken is a candidate path: an optional drive, then path
// characters ending in an extension that starts with a letter, then an
// optional :line.
var briefPathToken = regexp.MustCompile(`(?:[A-Za-z]:)?[A-Za-z0-9_.\-/\\]+\.[A-Za-z][A-Za-z0-9]{0,7}(?::(\d+))?`)

// briefSites measures the edit sites a brief names. A candidate is a site
// when it names a directory (a/b.go) or a file that exists in the workspace
// (b.go): a qualified identifier (fmt.Errorf, errors.New) has neither, and an
// absolute path outside the workspace is not the workspace's to edit. Sites
// are distinct by path and line, so a brief that names one file at two lines
// names two sites. Go measures this; whether it is enough sites to plan is
// the policy's (turn_needs_step_plan).
func briefSites(workspace, brief string) []briefSite {
	text := briefURL.ReplaceAllString(brief, " ")
	var sites []briefSite
	seen := map[briefSite]bool{}
	for _, m := range briefPathToken.FindAllStringSubmatch(text, -1) {
		token := m[0]
		var line int64
		if m[1] != "" {
			token = strings.TrimSuffix(token, ":"+m[1])
			line, _ = strconv.ParseInt(m[1], 10, 64)
		}
		rel, ok := briefWorkspacePath(workspace, token)
		if !ok {
			continue
		}
		site := briefSite{Path: rel, Line: line}
		if !seen[site] {
			seen[site] = true
			sites = append(sites, site)
		}
	}
	return sites
}

// briefWorkspacePath resolves a candidate token to a workspace-relative,
// slash-separated path, or reports that it names no workspace file.
func briefWorkspacePath(workspace, token string) (string, bool) {
	p := filepath.ToSlash(token)
	if filepath.IsAbs(token) || filepath.IsAbs(p) {
		if workspace == "" {
			return "", false
		}
		rel, err := filepath.Rel(workspace, filepath.FromSlash(p))
		if err != nil || rel == ".." || strings.HasPrefix(filepath.ToSlash(rel), "../") {
			return "", false
		}
		p = filepath.ToSlash(rel)
	}
	p = strings.TrimPrefix(p, "./")
	if p == "" || strings.HasPrefix(p, "../") {
		return "", false
	}
	if strings.Contains(p, "/") {
		return p, true
	}
	if workspace == "" {
		return "", false
	}
	if st, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(p))); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}

// briefNeedsStepPlan asserts the brief's edit sites for this turn and asks
// the policy whether the turn is divided into planned steps
// (turn_needs_step_plan). No kernel, or a failed query, is one pass: the
// planning call is spent only where the policy derives it.
func (e *Executor) briefNeedsStepPlan(brief string, result *ExecutionResult) bool {
	if e.kernel == nil {
		return false
	}
	sites := briefSites(e.workspaceForVerification(), brief)
	turn := result.turnAtom()
	e.assertTurnVerb(turn, result.Intent.Verb, result)
	for _, s := range sites {
		e.assertTurnFact(types.Fact{Predicate: "turn_brief_site", Args: []any{turn, types.MangleString(s.Path), s.Line}})
	}
	e.ensureSessionParams()
	rows, err := e.kernel.Query("turn_needs_step_plan")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("turn_needs_step_plan query failed (%v); the task runs as one pass", err)
		return false
	}
	for _, f := range rows {
		if len(f.Args) == 1 && types.ExtractString(f.Args[0]) == string(turn) {
			return true
		}
	}
	logging.SessionDebug("The brief names %d edit site(s) and the policy derives no step plan; the task runs as one pass", len(sites))
	return false
}
