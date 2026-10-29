package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// TestGrepGlobListFilesHonorGitignore is the tool-facing half of membership:
// a gitignored tree is not a result, a dot directory git does not ignore is,
// and a default world.ignore_patterns name (node_modules) is dropped even
// when .gitignore does not mention it.
func TestGrepGlobListFilesHonorGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	token := "MEMBER_" + "TOKEN_keep"
	ignoredToken := "IGNORED_" + "TOKEN_secret"
	hiddenToken := "HIDDEN_" + "TOKEN_dot"
	depToken := "DEP_" + "TOKEN_nm"
	write(".gitignore", "secret/\n")
	write("keep.go", "package keep\n// "+token+"\n")
	write(".hidden/h.go", "package hidden\n// "+hiddenToken+"\n")
	write("secret/a.go", "package secret\n// "+ignoredToken+"\n")
	write("node_modules/lib/x.go", "package lib\n// "+depToken+"\n")

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.email=test@example.com",
			"-c", "user.name=test",
			"-c", "commit.gpgsign=false",
		}, args...)...)
		cmd.Dir = root
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "GIT_DIR=") || strings.HasPrefix(e, "GIT_WORK_TREE=") {
				continue
			}
			env = append(env, e)
		}
		cmd.Env = append(env, "GIT_OPTIONAL_LOCKS=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	guardPath := "internal/build/no_hardcoded_models_test.go"
	guardToken := "TRACKED_GUARD_" + "TOKEN"
	write(guardPath, "package build\n// "+guardToken+"\nfunc GuardVisible() {}\n")
	outputToken := "OUTPUT_" + "TOKEN"
	write("build/generated.go", "package output\n// "+outputToken+"\n")
	write("nested/build/generated.go", "package output\n// "+outputToken+"\n")
	git("init")
	git("add", ".gitignore", "keep.go", ".hidden/h.go", guardPath)
	git("commit", "-m", "init")

	t.Setenv("CODENERD_WORKSPACE_ROOT", root)
	t.Chdir(root)
	ctx := tools.WithWorkspaceRoot(context.Background(), root)

	for name, check := range map[string]struct {
		run  func(context.Context, map[string]any) (string, error)
		args map[string]any
		want string
	}{
		"read":        {executeReadFile, map[string]any{"path": guardPath}, guardToken},
		"glob":        {executeGlob, map[string]any{"pattern": "**/*hardcoded*"}, "no_hardcoded_models_test.go"},
		"grep":        {executeGrep, map[string]any{"pattern": guardToken}, "no_hardcoded_models_test.go"},
		"search_code": {executeSearchCode, map[string]any{"pattern": "GuardVisible"}, "GuardVisible"},
		"list":        {executeListFiles, map[string]any{"path": "internal/build"}, "no_hardcoded_models_test.go"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := check.run(ctx, check.args)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, check.want) || strings.Contains(out, "not workspace files") {
				t.Fatalf("%s missed tracked guard: %s", name, out)
			}
		})
	}
	for _, search := range []func(context.Context, map[string]any) (string, error){executeGrep, executeSearchCode} {
		out, err := search(ctx, map[string]any{"pattern": outputToken})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "No matches found") {
			t.Fatalf("search returned build output: %s", out)
		}
	}
	for _, walk := range []struct {
		run  func(context.Context, map[string]any) (string, error)
		args map[string]any
	}{
		{executeGlob, map[string]any{"pattern": "**/*.go"}},
		{executeListFiles, map[string]any{"recursive": true}},
	} {
		out, err := walk.run(ctx, walk.args)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "generated.go") {
			t.Fatalf("walk returned build output: %s", out)
		}
	}

	grepOut, err := executeGrep(ctx, map[string]any{"pattern": token})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(grepOut, "keep.go") {
		t.Fatalf("grep missed keep.go: %s", grepOut)
	}
	for _, banned := range []string{ignoredToken, depToken} {
		out, err := executeGrep(ctx, map[string]any{"pattern": banned})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "No matches found") {
			t.Fatalf("grep returned ignored token %s: %s", banned, out)
		}
	}
	hiddenOut, err := executeGrep(ctx, map[string]any{"pattern": hiddenToken})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hiddenOut, hiddenToken) {
		t.Fatalf("grep missed tracked .hidden file: %s", hiddenOut)
	}

	globOut, err := executeGlob(ctx, map[string]any{"pattern": "**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(globOut, "keep.go") || !strings.Contains(globOut, ".hidden") {
		t.Fatalf("glob missed members: %s", globOut)
	}
	if strings.Contains(globOut, "secret") || strings.Contains(globOut, "node_modules") {
		t.Fatalf("glob returned ignored files: %s", globOut)
	}

	// An ignored match is not a result, but the empty answer says it exists:
	// "No files found" read as "the file does not exist".
	for _, pattern := range []string{"secret/*.go", "secret/**/*.go"} {
		out, err := executeGlob(ctx, map[string]any{"pattern": pattern})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "No workspace files match") || !strings.Contains(out, "secret") || strings.Contains(out, ignoredToken) {
			t.Fatalf("glob %s does not say its matches are ignored: %s", pattern, out)
		}
	}

	listOut, err := executeListFiles(ctx, map[string]any{
		"path":           ".",
		"recursive":      true,
		"include_hidden": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, "keep.go") || !strings.Contains(listOut, ".hidden") {
		t.Fatalf("list_files missed members: %s", listOut)
	}
	if strings.Contains(listOut, "secret") || strings.Contains(listOut, "node_modules") {
		t.Fatalf("list_files returned ignored trees: %s", listOut)
	}
}
