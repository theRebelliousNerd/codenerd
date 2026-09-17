package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/processutil"
)

// -----------------------------------------------------------------------------
// System facts: wall clock and git working-tree state, refreshed into the EDB.
// -----------------------------------------------------------------------------

// UpdateSystemFacts refreshes system-level facts: wall-clock time always,
// plus git working-tree state when the workspace root is a git checkout.
// Git fact groups are gated on their own commands; failures skip facts,
// never assert them.
func (k *RealKernel) UpdateSystemFacts() error {
	// Nil receiver is an error, not a crash (see Query for the interface-trap rationale).
	if k == nil {
		return fmt.Errorf("updateSystemFacts: kernel is nil")
	}
	now := time.Now().Unix()

	tx := k.Transaction()
	tx.Retract("current_time")
	tx.Assert(Fact{Predicate: "current_time", Args: []any{now}})

	workspaceRoot := strings.TrimSpace(k.workspaceRoot)
	if workspaceRoot == "" {
		logging.KernelDebug("UpdateSystemFacts: workspace root not set, skipping git facts")
		return tx.Commit()
	}
	if abs, err := filepath.Abs(workspaceRoot); err == nil {
		workspaceRoot = abs
	}
	if info, err := os.Stat(workspaceRoot); err != nil || !info.IsDir() {
		logging.KernelDebug("UpdateSystemFacts: invalid workspace root: %s", workspaceRoot)
		return tx.Commit()
	}

	gitRoot, err := gitRepoRoot(workspaceRoot)
	if err != nil {
		logging.KernelDebug("UpdateSystemFacts: git root not found: %v", err)
		return tx.Commit()
	}

	branch, branchErr := gitCmd(gitRoot, "rev-parse", "--abbrev-ref", "HEAD")
	statusOutput, statusErr := gitCmd(gitRoot, "status", "--porcelain")
	commitOutput, commitErr := gitCmd(gitRoot, "log", "-n", "5", "--pretty=format:%s")

	// Each fact group is gated on its own command. Parsing failed output
	// would assert lies — notably unstaged_count("0") when status errored.
	// A fresh repo (unborn HEAD) fails rev-parse/log but passes status,
	// so the groups stay independent instead of all-or-nothing.
	var modifiedFiles, untrackedFiles []string
	unstagedCount := 0
	statusOK := statusErr == nil
	if statusOK {
		modifiedFiles, unstagedCount, untrackedFiles = parseGitStatus(statusOutput)
	} else {
		logging.KernelDebug("UpdateSystemFacts: git status failed: %v", statusErr)
	}
	var recentCommits []string
	if commitErr == nil {
		recentCommits = splitLinesTrimmed(commitOutput)
	} else {
		logging.KernelDebug("UpdateSystemFacts: git log failed: %v", commitErr)
	}
	if branchErr != nil {
		logging.KernelDebug("UpdateSystemFacts: git rev-parse failed: %v", branchErr)
		branch = ""
	}

	tx.Retract("git_state")
	tx.Retract("git_branch")

	if branch != "" {
		tx.Assert(Fact{Predicate: "git_state", Args: []any{"/branch", branch}})
		tx.Assert(Fact{Predicate: "git_branch", Args: []any{branch}})
	}
	if statusOK && len(modifiedFiles) > 0 {
		tx.Assert(Fact{Predicate: "git_state", Args: []any{"/modified_files", strings.Join(modifiedFiles, "\n")}})
	}
	if commitErr == nil && len(recentCommits) > 0 {
		tx.Assert(Fact{Predicate: "git_state", Args: []any{"/recent_commits", strings.Join(recentCommits, "\n")}})
	}
	if statusOK {
		tx.Assert(Fact{Predicate: "git_state", Args: []any{"/unstaged_count", strconv.Itoa(unstagedCount)}})
	}
	// The new_files world state gates a mandatory prompt atom whose own text
	// reads "Untracked files exist in the working directory". Nothing had ever
	// set it: CompilationContext.HasNewFiles had no writer anywhere, so that
	// atom could not be selected in any session. The status output this
	// function already parses is where the answer was.
	if statusOK && len(untrackedFiles) > 0 {
		tx.Assert(Fact{Predicate: "git_state", Args: []any{"/untracked_files", strings.Join(untrackedFiles, "\n")}})
	}

	return tx.Commit()
}

func gitRepoRoot(workspaceRoot string) (string, error) {
	out, err := gitCmd(workspaceRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}

func gitCmd(workspaceRoot string, args ...string) (string, error) {
	if workspaceRoot == "" {
		return "", fmt.Errorf("workspace root is empty")
	}

	if len(args) == 0 {
		return "", fmt.Errorf("no git subcommand provided")
	}

	// Validate subcommand
	validSubcommands := map[string]bool{
		"rev-parse": true,
		"status":    true,
		"log":       true,
	}
	if !validSubcommands[args[0]] {
		return "", fmt.Errorf("unauthorized git subcommand: %s", args[0])
	}

	// Defensive check against argument injection for dangerous flags
	for _, arg := range args {
		if strings.HasPrefix(arg, "--exec-path") ||
			strings.HasPrefix(arg, "-c") ||
			strings.HasPrefix(arg, "--upload-pack") ||
			strings.HasPrefix(arg, "--receive-pack") {
			return "", fmt.Errorf("unauthorized git argument: %s", arg)
		}
	}

	cmd := processutil.NonInteractive(exec.Command("git", append([]string{"-C", workspaceRoot}, args...)...))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// splitPorcelainLines splits `git status --porcelain` output WITHOUT trimming
// the leading whitespace, because in that format the leading whitespace is data.
//
// Porcelain lines are "XY path", where X is the index column and Y the worktree
// column, and a space is a meaningful value in either. " M file" means modified
// in the worktree and not staged; "M  file" means staged. Trimming the line
// shifts both columns left and turns the first into the second, so the two
// became indistinguishable and — since the check is on the SECOND column — an
// ordinary edited-but-unstaged file was counted as staged.
//
// That is the most common state a working tree is ever in, so the unstaged count
// only ever saw untracked files and files staged and then edited again. The
// high_churn world state is gated on that count exceeding twenty, and could not
// be reached by editing files at all.
func splitPorcelainLines(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		// Trailing only: \r from a Windows git, and any trailing spaces. The
		// leading columns stay exactly as git wrote them.
		part = strings.TrimRight(part, " \t\r")
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseGitStatus returns the changed paths, the unstaged count, and the
// untracked paths.
//
// The untracked set was already being recognised here -- the "??" branch
// below has always identified it -- and then folded into the unstaged count
// and discarded. It is returned separately because it answers a different
// question: an untracked file is not a change to something that exists, it is
// something that exists and is not yet part of the project.
func parseGitStatus(statusOutput string) (files []string, unstaged int, untracked []string) {
	lines := splitPorcelainLines(statusOutput)
	files = make([]string, 0, len(lines))

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[2:])
		if path == "" {
			continue
		}
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = strings.TrimSpace(parts[len(parts)-1])
		}
		files = append(files, path)

		if status == "??" {
			unstaged++
			untracked = append(untracked, path)
			continue
		}
		if len(status) == 2 && status[1] != ' ' {
			unstaged++
		}
	}

	return dedupeStrings(files), unstaged, dedupeStrings(untracked)
}

func splitLinesTrimmed(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
