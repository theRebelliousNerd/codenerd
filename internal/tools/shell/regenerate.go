package shell

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"

	"codenerd/internal/build"
	"codenerd/internal/processutil"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// regenerateDefaultTimeout bounds a generator run that names no timeout. A
// corpus rebuild is minutes, not seconds.
const regenerateDefaultTimeout = 900

// RegenerateTool runs one generator the workspace's nerd.md declares.
//
// A change can leave a checked-in generated file stale (a corpus built from
// YAML, a ledger scored from atoms), and the shell gate refuses every command
// it does not recognize, so an agent could name the repair and not run it.
// This tool is the governed path: it runs only a declared generator, its
// writes are exactly the generator's declared outputs (ExpandArgs puts them in
// the call's paths, so the write guards, the preimage capture and the turn's
// write set see them like any write), and a run that changes any other file
// fails by name.
func RegenerateTool() *tools.Tool {
	return &tools.Tool{
		Name: "regenerate",
		Description: "Run one of the workspace's declared generators (nerd.md generators) to rewrite the checked-in generated files it owns, " +
			"e.g. after editing the sources a corpus or ledger is built from. Pass its id; its outputs are the call's writes. " +
			"A check that fails because a generated file is stale names the generator to run",
		Category:      tools.CategoryCode,
		AltCategories: []tools.ToolCategory{tools.CategoryTest},
		Priority:      70,
		Execute:       executeRegenerate,
		ExpandArgs:    expandRegenerateArgs,
		Timeout: func(args map[string]any) time.Duration {
			return time.Duration(regenerateTimeoutSeconds(args)) * time.Second
		},
		Schema: tools.ToolSchema{
			Required: []string{"id"},
			Properties: map[string]tools.Property{
				"id": {Type: "string", Description: "The generator's id in nerd.md"},
				"paths": {Type: "array", Description: "Filled in from the generator's declared outputs; leave it out",
					Items: &tools.PropertyItems{Type: "string"}},
				"timeout_seconds": {Type: "integer", Description: "Timeout in seconds (default: 900)", Default: regenerateDefaultTimeout},
			},
		},
	}
}

func regenerateTimeoutSeconds(args map[string]any) int {
	if t, ok := coerceInt(args["timeout_seconds"]); ok && t > 0 {
		return t
	}
	return regenerateDefaultTimeout
}

// declaredGenerator is the workspace's generator named by args.
func declaredGenerator(ctx context.Context, args map[string]any) (string, projectdoc.GeneratorSpec, *projectdoc.Document, error) {
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", projectdoc.GeneratorSpec{}, nil, err
	}
	id, _ := args["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return "", projectdoc.GeneratorSpec{}, nil, errors.New("regenerate needs the generator's id")
	}
	doc, err := projectdoc.Load(root)
	if err != nil {
		return "", projectdoc.GeneratorSpec{}, nil, fmt.Errorf("reading nerd.md: %w", err)
	}
	if doc == nil {
		return "", projectdoc.GeneratorSpec{}, nil, errors.New("this workspace has no nerd.md, so it declares no generators")
	}
	gen, ok := doc.Spec.Generator(id)
	if !ok {
		return "", projectdoc.GeneratorSpec{}, nil, fmt.Errorf("nerd.md declares no generator %q (declared: %s)", id, strings.Join(generatorIDs(doc), ", "))
	}
	return root, gen, doc, nil
}

func generatorIDs(doc *projectdoc.Document) []string {
	ids := make([]string, 0, len(doc.Spec.Generators))
	for _, g := range doc.Spec.Generators {
		ids = append(ids, g.ID)
	}
	if len(ids) == 0 {
		return []string{"none"}
	}
	return ids
}

// declaredOutputs is gen's outputs, cleaned, slash-separated and sorted.
func declaredOutputs(gen projectdoc.GeneratorSpec) []string {
	out := make([]string, 0, len(gen.Outputs))
	for _, p := range gen.Outputs {
		out = append(out, filepath.ToSlash(filepath.Clean(strings.TrimSpace(p))))
	}
	sort.Strings(out)
	return slices.Compact(out)
}

// expandRegenerateArgs puts the generator's declared outputs in the call's
// paths. Paths the caller supplied must be exactly those outputs.
func expandRegenerateArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	_, gen, _, err := declaredGenerator(ctx, args)
	if err != nil {
		return nil, err
	}
	outputs := declaredOutputs(gen)
	if supplied, present := args["paths"]; present {
		got, err := stringList(supplied)
		if err != nil {
			return nil, fmt.Errorf("regenerate paths: %w", err)
		}
		for i := range got {
			got[i] = filepath.ToSlash(filepath.Clean(got[i]))
		}
		sort.Strings(got)
		if !slices.Equal(slices.Compact(got), outputs) {
			return nil, fmt.Errorf("generator %q writes %s, not %s; leave paths out", gen.ID, strings.Join(outputs, ", "), strings.Join(got, ", "))
		}
	}
	expanded := make(map[string]any, len(args)+1)
	for k, v := range args {
		expanded[k] = v
	}
	paths := make([]any, len(outputs))
	for i, p := range outputs {
		paths[i] = p
	}
	expanded["paths"] = paths
	return expanded, nil
}

func stringList(v any) ([]string, error) {
	switch vs := v.(type) {
	case []string:
		return append([]string(nil), vs...), nil
	case []any:
		out := make([]string, 0, len(vs))
		for _, x := range vs {
			s, ok := x.(string)
			if !ok {
				return nil, errors.New("must be strings")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, errors.New("must be an array of strings")
	}
}

// RegenerateResult is the tool's report.
type RegenerateResult struct {
	ID       string   `json:"id"`
	Argv     []string `json:"argv"`
	ExitCode *int     `json:"exit_code,omitempty"`
	// Changed are the declared outputs whose bytes the run changed;
	// Unchanged the ones it left as they were.
	Changed   []string `json:"changed"`
	Unchanged []string `json:"unchanged"`
	// Undeclared are files outside the outputs the run changed. Any makes
	// the run a failure.
	Undeclared []string `json:"undeclared,omitempty"`
	// UndeclaredUnchecked says why other files could not be checked.
	UndeclaredUnchecked string `json:"undeclared_unchecked,omitempty"`
	Output              string `json:"output,omitempty"`
}

func executeRegenerate(ctx context.Context, args map[string]any) (string, error) {
	root, gen, doc, err := declaredGenerator(ctx, args)
	if err != nil {
		return "", err
	}
	outputs := declaredOutputs(gen)
	// Direct registry execution without the executor's expansion still runs
	// only what is declared: the paths, when present, must be the outputs.
	if _, err := expandRegenerateArgs(ctx, args); err != nil {
		return "", err
	}
	argv, err := shellquote.Split(gen.Run)
	if err != nil {
		return "", fmt.Errorf("nerd.md generator %q run %q: %w", gen.ID, gen.Run, err)
	}

	before, beforeErr := dirtyDigests(ctx, root)
	outputsBefore := fileDigests(root, outputs)

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(regenerateTimeoutSeconds(args))*time.Second)
	defer cancel()
	if len(argv) == 0 {
		return "", fmt.Errorf("nerd.md generator %q has an empty run", gen.ID)
	}
	cmd := build.WorkspaceCommand(runCtx, root, argv, doc.Spec.Commands.Env)
	var combined bytes.Buffer
	cmd.Stdout, cmd.Stderr = &combined, &combined
	runErr := processutil.Run(cmd)

	result := RegenerateResult{ID: gen.ID, Argv: argv, Output: strings.TrimSpace(combined.String())}
	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		result.ExitCode = &code
	}
	outputsAfter := fileDigests(root, outputs)
	for _, p := range outputs {
		if outputsBefore[p] != outputsAfter[p] {
			result.Changed = append(result.Changed, p)
		} else {
			result.Unchanged = append(result.Unchanged, p)
		}
	}
	declared := make(map[string]bool, len(outputs))
	for _, p := range outputs {
		declared[p] = true
	}
	after, afterErr := dirtyDigests(ctx, root)
	switch {
	case beforeErr != nil:
		result.UndeclaredUnchecked = beforeErr.Error()
	case afterErr != nil:
		result.UndeclaredUnchecked = afterErr.Error()
	default:
		for p, digest := range after {
			if !declared[p] && before[p] != digest {
				result.Undeclared = append(result.Undeclared, p)
			}
		}
		for p := range before {
			if _, still := after[p]; !still && !declared[p] {
				// Was dirty, is clean now: the run reverted someone's change.
				result.Undeclared = append(result.Undeclared, p)
			}
		}
		sort.Strings(result.Undeclared)
	}

	report, _ := json.Marshal(result)
	if runErr != nil {
		if ctxErr := runCtx.Err(); ctxErr != nil {
			return string(report), fmt.Errorf("generator %q stopped: %w", gen.ID, ctxErr)
		}
		return string(report), fmt.Errorf("generator %q failed: %w", gen.ID, runErr)
	}
	if len(result.Undeclared) > 0 {
		return string(report), fmt.Errorf("generator %q changed files it does not declare: %s", gen.ID, strings.Join(result.Undeclared, ", "))
	}
	return string(report), nil
}

// fileDigests is the content digest of each path ("" when absent).
func fileDigests(root string, paths []string) map[string]string {
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		out[p] = fileDigest(filepath.Join(root, filepath.FromSlash(p)))
	}
	return out
}

func fileDigest(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// dirtyDigests is every path git reports as modified, added, deleted or
// untracked in root, with its content digest: what differs from HEAD before
// and after a run. Outside a git work tree it is an error, and undeclared
// changes go unchecked (the report says so).
func dirtyDigests(ctx context.Context, root string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = root
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := processutil.Run(cmd); err != nil {
		return nil, fmt.Errorf("other files could not be checked: git status: %w", err)
	}
	out := stdout.Bytes()
	digests := map[string]string{}
	entries := strings.Split(string(out), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		status, path := e[:2], e[3:]
		if status[0] == 'R' || status[0] == 'C' {
			i++ // the rename's source follows
		}
		digests[filepath.ToSlash(path)] = fileDigest(filepath.Join(root, filepath.FromSlash(path)))
	}
	return digests, nil
}
