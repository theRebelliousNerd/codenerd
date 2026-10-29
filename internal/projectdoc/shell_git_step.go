package projectdoc

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kballard/go-shellquote"
)

// The steps of a git workflow a user's request can authorize. The names are
// the kernel's (policy/constitution.mg, section 7E: shell_call/3's Step).
const (
	GitStepAdd    = "git_add"
	GitStepCommit = "git_commit"
)

// GitStep is a shell call recognised as exactly one simple git step, as the
// executor would run it. It is a measurement of the call's shape, nothing
// more: whether the step is allowed is the kernel's.
type GitStep struct {
	// Step is GitStepAdd or GitStepCommit.
	Step string
	// Paths are the workspace-relative, slash-separated files a git_add stages.
	// Empty for a commit, whose paths are whatever is staged when it runs.
	Paths []string
}

// stepShellTools are the tools whose command text runs as one process when it
// carries no unquoted shell operator (tools/shell executeRunCommand splits it
// with shellquote and executes the argv directly). PowerShell and cmd tools
// have other quoting rules and are not recognised.
var stepShellTools = map[string]bool{
	"run_command": true, "bash": true, "sh": true, "shell": true, "exec": true, "run_shell": true,
}

// ParseGitStep recognises a tool call that is exactly `git add [--] <file>...`
// or `git commit -m <message>...`, and nothing wider: no other subcommand, no
// other flag (-A, -a, -f, -u, --amend, --no-verify, -c, ...), no pathspec
// magic or glob, no unquoted shell operator, no variable or command
// expansion. Any other shape returns an error naming what stopped it, and the
// caller leaves the call refused.
//
// git_operation (the typed git tool) is read from its own arguments the way
// its handler turns them into a command line, not from the summary string the
// classifier builds: a commit there is `-m <message>` and its other arguments
// are ignored, an add stages `files` or `args` and defaults to everything
// ("."), which no file list matches.
func ParseGitStep(toolName string, args map[string]any, workspace string) (GitStep, error) {
	name := strings.ToLower(strings.TrimSpace(toolName))
	if name == "git_operation" {
		return parseGitOperation(args, workspace)
	}
	if !stepShellTools[name] {
		return GitStep{}, fmt.Errorf("%s is not a tool whose command is recognised as one git step", toolName)
	}
	command := ShellCommand(args)
	if err := plainCommand(command); err != nil {
		return GitStep{}, err
	}
	tokens, err := shellquote.Split(command)
	if err != nil {
		return GitStep{}, fmt.Errorf("command does not parse: %v", err)
	}
	return parseGitTokens(tokens, workspace)
}

func parseGitOperation(args map[string]any, workspace string) (GitStep, error) {
	switch strings.ToLower(stringArg(args, "operation")) {
	case "add":
		list := stringArg(args, "files")
		if list == "" {
			list = stringArg(args, "args")
		}
		if list == "" {
			return GitStep{}, fmt.Errorf("git add names no files (it would stage everything)")
		}
		tokens, err := shellquote.Split(list)
		if err != nil {
			return GitStep{}, fmt.Errorf("file list does not parse: %v", err)
		}
		return parseAddPaths(tokens, workspace)
	case "commit":
		if stringArg(args, "message") == "" {
			return GitStep{}, fmt.Errorf("git commit has no message")
		}
		return GitStep{Step: GitStepCommit}, nil
	default:
		return GitStep{}, fmt.Errorf("git operation %q is not a recognised step", stringArg(args, "operation"))
	}
}

// plainCommand refuses command text the runner could hand to a shell, or in
// which a word could expand: an unquoted operator, a newline, a backquote, a
// substitution or a variable. The runner's compound test and the classifier's
// both mask quoted spans, so this uses the same mask.
func plainCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("no command payload")
	}
	masked, balanced := maskQuotedSegments(command)
	if !balanced {
		return fmt.Errorf("unbalanced quotes")
	}
	if strings.ContainsAny(masked, "\r\n;|&><") {
		return fmt.Errorf("the command chains or redirects; run one simple command per call")
	}
	if strings.ContainsAny(command, "`$\r\n\x00") {
		return fmt.Errorf("the command contains a substitution or variable")
	}
	return nil
}

func parseGitTokens(tokens []string, workspace string) (GitStep, error) {
	if len(tokens) < 2 || tokens[0] != "git" {
		return GitStep{}, fmt.Errorf("the command is not `git add` or `git commit`")
	}
	switch tokens[1] {
	case "add":
		return parseAddPaths(tokens[2:], workspace)
	case "commit":
		rest := tokens[2:]
		if len(rest) == 0 || len(rest)%2 != 0 {
			return GitStep{}, fmt.Errorf("git commit takes only `-m <message>`")
		}
		for i := 0; i < len(rest); i += 2 {
			if rest[i] != "-m" || strings.TrimSpace(rest[i+1]) == "" {
				return GitStep{}, fmt.Errorf("git commit takes only `-m <message>` (no -a, --amend, --no-verify or other flags)")
			}
		}
		return GitStep{Step: GitStepCommit}, nil
	default:
		return GitStep{}, fmt.Errorf("git %s is not a recognised step", tokens[1])
	}
}

// parseAddPaths reads the words after `git add`: an optional leading `--`,
// then one or more plain files. A flag, a glob, pathspec magic, "." or a path
// outside the workspace is not a file this call can be said to stage.
func parseAddPaths(words []string, workspace string) (GitStep, error) {
	if len(words) > 0 && words[0] == "--" {
		words = words[1:]
	}
	if len(words) == 0 {
		return GitStep{}, fmt.Errorf("git add names no files")
	}
	paths := make([]string, 0, len(words))
	for _, w := range words {
		if strings.HasPrefix(w, "-") || strings.HasPrefix(w, ":") || strings.ContainsAny(w, "*?[\\") {
			return GitStep{}, fmt.Errorf("git add takes plain file names, not %q", w)
		}
		rel, ok := workspaceRelative(w, workspace)
		if !ok {
			return GitStep{}, fmt.Errorf("git add path %q is not a file inside the workspace", w)
		}
		paths = append(paths, rel)
	}
	return GitStep{Step: GitStepAdd, Paths: paths}, nil
}

// workspaceRelative is path as a clean, slash-separated, workspace-relative
// file name; false for the workspace itself, a path that leaves it, or an
// empty one.
func workspaceRelative(path, workspace string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		if workspace == "" {
			return "", false
		}
		rel, err := filepath.Rel(filepath.Clean(workspace), clean)
		if err != nil {
			return "", false
		}
		clean = rel
	}
	clean = filepath.ToSlash(clean)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}
