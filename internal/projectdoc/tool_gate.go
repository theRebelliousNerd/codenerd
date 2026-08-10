package projectdoc

import (
	"fmt"
	"strings"
)

// The mutation classification and target extraction live here so every gate
// shares one definition. Caller-only copies previously let direct registry
// execution bypass project policy.

// PathArgs are the argument names a write-mutation tool may use to name its
// target. Tools disagree ("path", "file_path", "file", "filename"), so every
// gate checks all of them rather than trusting one convention.
var PathArgs = []string{"path", "file_path", "filepath", "file", "filename", "target", "dest", "destination"}

// TargetPath extracts the target path from a tool call's arguments, or "" when
// none of the known argument names carry one.
// MaxNestedEdits bounds the number of nested edit targets extracted from an
// edits array. It is enforced as a hard limit and rejected rather than
// silently truncated so a caller cannot believe 16 edits landed while 17
// were dropped.
const MaxNestedEdits = 16

// TargetPaths extracts all target paths from args in a deterministic,
// ordered-deduplicated form. It preserves the legacy single-file scalar
// behavior (first matching PathArgs key) and additionally extracts path
// fields from an "edits" array of objects. Each edit object is searched
// with the same PathArgs keys used for the top-level. The extraction is
// bounded to MaxNestedEdits entries and returns an error on malformed or
// oversize input rather than silently dropping data.
func TargetPaths(args map[string]any) ([]string, error) {
	if args == nil {
		return nil, nil
	}
	seen := make(map[string]struct{}, 4)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, key := range PathArgs {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				add(s)
				break
			}
		}
	}
	if rawEdits, ok := args["edits"]; ok && rawEdits != nil {
		nested, err := extractNestedPaths(rawEdits)
		if err != nil {
			return nil, err
		}
		for _, p := range nested {
			add(p)
		}
	}
	return out, nil
}

// extractNestedPaths validates and extracts path fields from the edits
// value. It is the error-returning validation helper that enforces the
// 16-edit bound and rejects malformed input.
func extractNestedPaths(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	var elems []any
	switch v := raw.(type) {
	case []any:
		elems = v
	case []map[string]any:
		elems = make([]any, len(v))
		for i, m := range v {
			elems[i] = m
		}
	default:
		return nil, fmt.Errorf("edits must be an array of objects")
	}
	if len(elems) > MaxNestedEdits {
		return nil, fmt.Errorf("edits exceeds maximum of %d entries (%d)", MaxNestedEdits, len(elems))
	}
	var out []string
	for i, elem := range elems {
		if elem == nil {
			return nil, fmt.Errorf("edits[%d] is null", i)
		}
		m, ok := elem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("edits[%d] must be an object", i)
		}
		found := ""
		for _, key := range PathArgs {
			if rawPath, ok := m[key]; ok {
				if s, ok := rawPath.(string); ok {
					if strings.TrimSpace(s) != "" {
						found = strings.TrimSpace(s)
						break
					}
					return nil, fmt.Errorf("edits[%d].%s must be a non-empty string", i, key)
				}
				return nil, fmt.Errorf("edits[%d].%s must be a string", i, key)
			}
		}
		if found == "" {
			return nil, fmt.Errorf("edits[%d] is missing a target path (%s)", i, strings.Join(PathArgs, "/"))
		}
		out = append(out, found)
	}
	return out, nil
}

// TargetPath extracts the target path from a tool call's arguments, or "" when
// none of the known argument names carry one. It is the first-target
// compatibility wrapper over TargetPaths; malformed nested input is treated
// as no target so callers that only understand single paths fail closed via
// the "no targets" path.
func TargetPath(args map[string]any) string {
	paths, err := TargetPaths(args)
	if err != nil || len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// IsWriteMutationTool reports whether a tool name durably mutates a file.
func IsWriteMutationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case // Registered VirtualStore write actions.
		"write_file", "edit_file", "delete_file",
		"edit_lines", "insert_lines", "delete_lines",
		"edit_element", "apply_edits", "fs_write",
		// Defensive aliases.
		"apply_patch", "str_replace", "create_file", "replace_in_file", "multi_edit":
		return true
	default:
		return false
	}
}

// IsShellTool reports whether a tool name can route a command to a host shell
// or process executor. run_command is the registered modular tool; the others
// cover VirtualStore action aliases and defensive future registry names.
func IsShellTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "run_command", "bash", "sh", "pwsh", "powershell", "cmd",
		"shell", "exec", "run_shell", "run_build", "run_tests",
		"git_diff", "git_log", "git_operation":
		return true
	default:
		return false
	}
}

// ShellEffectKind is a bounded classification of a shell invocation.
type ShellEffectKind int

const (
	ShellEffectNone ShellEffectKind = iota
	ShellEffectReadOnly
	ShellEffectVerification
	ShellEffectMutating
	ShellEffectUnknownMutating
)

func (k ShellEffectKind) String() string {
	switch k {
	case ShellEffectNone:
		return "none"
	case ShellEffectReadOnly:
		return "read_only"
	case ShellEffectVerification:
		return "verification"
	case ShellEffectMutating:
		return "mutating"
	case ShellEffectUnknownMutating:
		return "unknown_mutating"
	default:
		return "unknown"
	}
}

// IsMutating reports whether a command is known or conservatively assumed to
// mutate. A missing command is denied separately by ValidateShellToolInvocation.
func (k ShellEffectKind) IsMutating() bool {
	return k == ShellEffectMutating || k == ShellEffectUnknownMutating
}

// AllowedWithoutMutationScope reports the two command classes that remain
// usable before baseline-aware shell authorization exists. Verification is a
// separate class because tests and builds may execute code even though the user
// explicitly requires them to remain available.
func (k ShellEffectKind) AllowedWithoutMutationScope() bool {
	return k == ShellEffectReadOnly || k == ShellEffectVerification
}

// ShellCommand extracts the command payload from a shell-capable tool call.
// Alternate keys cover defensive aliases; run_command itself uses "command".
func ShellCommand(args map[string]any) string {
	for _, key := range []string{"command", "script", "cmd", "content"} {
		if raw, ok := args[key]; ok {
			if command, ok := raw.(string); ok && strings.TrimSpace(command) != "" {
				return command
			}
		}
	}
	return ""
}

// ValidateShellToolInvocation is the shared fail-closed decision used by the
// session executor and the lowest registry guard. It classifies effects but
// never infers permission from command text. Until immutable task baseline and
// exact-path scope authority are wired, every shell mutation is refused.
func ValidateShellToolInvocation(toolName string, args map[string]any) (ShellEffectKind, string, error) {
	if !IsShellTool(toolName) {
		return ShellEffectNone, "", nil
	}

	normalizedName := strings.ToLower(strings.TrimSpace(toolName))
	// Dedicated verification tools are classified by identity. They resolve to
	// verification when absent or when they are a recognised build/test
	// invocation, but still refuse obviously mutating commands (knownMutations).
	if normalizedName == "run_build" || normalizedName == "run_tests" {
		rawCmd := ShellCommand(args)
		var command string
		if normalizedName == "run_build" && rawCmd == "" {
			return ShellEffectVerification, normalizedName + " (auto-detected)", nil
		}
		if normalizedName == "run_tests" && rawCmd == "" {
			// The concrete test runner is selected later from project files. A
			// synthetic verification prefix lets us still reject injected pattern
			// syntax before that selection happens.
			command = joinStructuredCommand("go test", args, "pattern")
			if strings.TrimSpace(command) == "go test" {
				return ShellEffectVerification, normalizedName + " (auto-detected)", nil
			}
		} else {
			command = structuredShellCommand(normalizedName, args)
			if strings.TrimSpace(command) == "" {
				return ShellEffectVerification, normalizedName + " (auto-detected)", nil
			}
		}
		// Refuse obviously mutating payloads even when smuggled via verification tools.
		lower := strings.ToLower(command)
		for _, marker := range knownMutations {
			if strings.Contains(lower, marker) {
				kind := ShellEffectMutating
				return kind, command, fmt.Errorf(
					"blocked by shell-effect gate: %s is %s and lacks deterministic task-scope authorization",
					toolName, kind,
				)
			}
		}
		kind := ClassifyShellEffect(command)
		if kind.AllowedWithoutMutationScope() {
			return kind, command, nil
		}
		if kind == ShellEffectNone {
			return kind, command, fmt.Errorf("blocked by shell-effect gate: %s has no command payload", toolName)
		}
		return kind, command, fmt.Errorf(
			"blocked by shell-effect gate: %s is %s and lacks deterministic task-scope authorization",
			toolName, kind,
		)
	}
	command := structuredShellCommand(normalizedName, args)
	kind := ClassifyShellEffect(command)
	if kind.AllowedWithoutMutationScope() {
		return kind, command, nil
	}
	if kind == ShellEffectNone {
		return kind, command, fmt.Errorf("blocked by shell-effect gate: %s has no command payload", toolName)
	}
	return kind, command, fmt.Errorf(
		"blocked by shell-effect gate: %s is %s and lacks deterministic task-scope authorization",
		toolName, kind,
	)
}

func structuredShellCommand(toolName string, args map[string]any) string {
	switch toolName {
	case "git_diff":
		return joinStructuredCommand("git diff", args, "commit", "path")
	case "git_log":
		return joinStructuredCommand("git log", args, "format", "since", "author", "path")
	case "git_operation":
		return joinStructuredCommand("git "+stringArg(args, "operation"), args, "args", "message", "branch", "files")
	case "run_tests":
		return joinStructuredCommand(ShellCommand(args), args, "pattern")
	default:
		return ShellCommand(args)
	}
}

func joinStructuredCommand(base string, args map[string]any, keys ...string) string {
	parts := []string{strings.TrimSpace(base)}
	for _, key := range keys {
		if value := stringArg(args, key); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func stringArg(args map[string]any, key string) string {
	if raw, ok := args[key]; ok {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var knownMutations = []string{
	"git add", "git am", "git apply", "git checkout", "git cherry-pick",
	"git clean", "git commit", "git fetch", "git merge", "git mv",
	"git pull", "git push", "git rebase", "git reset", "git restore",
	"git rm", "git stash", "git switch", "git tag",
	"rm ", "rm\t", "rmdir", "shutil.rmtree", "rmtree", ".unlink(",
	" unlink(", "os.remove", "os.unlink", "del ", "erase ",
	"remove-item", "clear-content", "set-content", "add-content",
	"new-item", "move-item", "copy-item", "rename-item", "out-file",
	"export-csv", "set-item", "set-acl",
}

var safePipeReaders = map[string]struct{}{
	"head": {}, "tail": {}, "cat": {}, "wc": {}, "grep": {}, "rg": {}, "sort": {}, "uniq": {},
}

func stripBenignOutputTail(command string) string {
	s := strings.TrimSpace(command)
	for {
		trimmed := strings.TrimSpace(s)
		if strings.HasSuffix(trimmed, "2>&1") {
			s = strings.TrimSpace(strings.TrimSuffix(trimmed, "2>&1"))
			continue
		}
		idx := strings.LastIndex(trimmed, "|")
		if idx == -1 {
			break
		}
		tail := strings.TrimSpace(trimmed[idx+1:])
		if tail == "" {
			break
		}
		if strings.ContainsAny(tail, "\r\n;`") || strings.Contains(tail, "$(") {
			break
		}
		if strings.ContainsAny(tail, "&><") {
			break
		}
		fields := strings.Fields(tail)
		if len(fields) == 0 {
			break
		}
		first := strings.ToLower(fields[0])
		if _, ok := safePipeReaders[first]; !ok {
			break
		}
		s = strings.TrimSpace(trimmed[:idx])
	}
	return s
}

// ClassifyShellEffect applies a deliberately small allowlist. Compound syntax,
// output flags, known mutation verbs, and every unrecognized command fail
// closed. The exact incident commands are classified as mutating before any
// read-only prefix is considered.
func ClassifyShellEffect(command string) ShellEffectKind {
	raw := strings.TrimSpace(command)
	if raw == "" {
		return ShellEffectNone
	}
	// Strip benign output-redirection tails for classification purposes only.
	// This allows verification commands like "go build ./... 2>&1" or
	// "go test ./... | head" to remain verification.
	stripped := stripBenignOutputTail(raw)
	lower := strings.ToLower(stripped)

	if strings.ContainsAny(stripped, "\r\n;|&><`") || strings.Contains(lower, "$(") {
		return ShellEffectUnknownMutating
	}

	for _, marker := range knownMutations {
		if strings.Contains(lower, marker) {
			return ShellEffectMutating
		}
	}

	// These options make otherwise observational commands write files or invoke
	// external helpers.
	for _, marker := range []string{"--output", "--ext-diff", "--textconv", " -o "} {
		if strings.Contains(lower, marker) {
			return ShellEffectUnknownMutating
		}
	}

	readOnly := []string{
		"git status", "git diff", "git log", "git show", "git ls-files",
		"git rev-parse", "git grep", "git blame", "git cat-file",
		"git worktree list", "git branch --show-current", "git branch --list",
		"ls", "dir", "tree", "cat", "head", "tail", "grep", "rg", "wc",
		"pwd", "whoami", "echo", "python --version", "python3 --version",
		"node --version", "get-childitem", "get-content", "get-item",
		"get-location", "test-path", "resolve-path", "select-string",
		"write-output",
	}
	for _, prefix := range readOnly {
		if hasCommandPrefix(lower, prefix) {
			return ShellEffectReadOnly
		}
	}

	verification := []string{
		"go test", "go vet", "go list", "go version", "go build",
		"cargo test", "cargo check", "cargo build", "npm test", "npm run build",
		"pytest", "python -m pytest", "python3 -m pytest", "dotnet test", "dotnet build",
		"mvn test", "mvn package", "gradle test", "gradle build", "./gradlew test",
		"./gradlew build", "cmake --build", "ctest", "bazel test", "bazel build",
	}
	for _, prefix := range verification {
		if hasCommandPrefix(lower, prefix) {
			return ShellEffectVerification
		}
	}

	return ShellEffectUnknownMutating
}

func hasCommandPrefix(command, prefix string) bool {
	if command == prefix {
		return true
	}
	return strings.HasPrefix(command, prefix) && len(command) > len(prefix) &&
		(command[len(prefix)] == ' ' || command[len(prefix)] == '\t')
}
