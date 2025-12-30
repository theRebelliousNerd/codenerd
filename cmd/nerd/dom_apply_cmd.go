package main

import (
	"context"
	"codenerd/internal/core"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	domApplyGoFmt  bool
	domApplyGoTest bool
	domApplyDryRun bool
)

type domApplyPlan struct {
	Edits []struct {
		Ref         string `json:"ref"`
		Content     string `json:"content,omitempty"`
		ContentFile string `json:"content_file,omitempty"`
	} `json:"edits"`
}

var domApplyCmd = &cobra.Command{
	Use:   "apply <file> <plan.json>",
	Short: "Apply a multi-element Code DOM plan within the file's 1-hop scope",
	Args:  cobra.ExactArgs(2),
	RunE:  runDomApply,
}

func init() {
	domCmd.AddCommand(domApplyCmd)
	domApplyCmd.Flags().BoolVar(&domApplyGoFmt, "gofmt", false, "Run gofmt on touched files after apply")
	domApplyCmd.Flags().BoolVar(&domApplyGoTest, "test", false, "Run `go test` in workspace after apply")
	domApplyCmd.Flags().BoolVar(&domApplyDryRun, "dry-run", false, "Validate plan and show what would change without writing files")
	domApplyCmd.Flags().IntVar(&domDemoDeepWorker, "deep-workers", 0, "Deep fact workers (0=auto)")
}

func runDomApply(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	target := args[0]
	planPath := args[1]

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	absPlan, err := filepath.Abs(planPath)
	if err != nil {
		return fmt.Errorf("resolve plan path: %w", err)
	}

	ws := workspace
	if ws == "" {
		if root, err := findGoWorkspaceRoot(absTarget); err == nil && root != "" {
			ws = root
		} else {
			ws, _ = os.Getwd()
		}
	}
	ws, _ = filepath.Abs(ws)

	planBytes, err := os.ReadFile(absPlan)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	var plan domApplyPlan
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return fmt.Errorf("parse plan json: %w", err)
	}
	if len(plan.Edits) == 0 {
		return fmt.Errorf("plan has no edits")
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	_, vs, scope, err := newDOMHarness(ws, domDemoDeepWorker)
	if err != nil {
		return err
	}

	fmt.Printf("Workspace: %s\n", ws)
	fmt.Printf("Opening:   %s\n", absTarget)

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", absTarget}}); err != nil {
		return fmt.Errorf("open_file failed: %w", err)
	}

	// Resolve edits and collect touched files.
	var payloadEdits []interface{}
	touchedFiles := make(map[string]struct{})
	for i, e := range plan.Edits {
		ref := strings.TrimSpace(e.Ref)
		if ref == "" {
			return fmt.Errorf("plan.edits[%d] missing ref", i)
		}

		content := e.Content
		if strings.TrimSpace(e.ContentFile) != "" {
			if strings.TrimSpace(content) != "" {
				return fmt.Errorf("plan.edits[%d] has both content and content_file", i)
			}
			b, err := os.ReadFile(e.ContentFile)
			if err != nil {
				return fmt.Errorf("read plan.edits[%d].content_file: %w", i, err)
			}
			content = string(b)
		}

		// Validate element exists and capture its file for gofmt.
		elem := scope.GetCoreElement(ref)
		if elem == nil {
			return fmt.Errorf("element not found in scope: %s", ref)
		}
		touchedFiles[elem.File] = struct{}{}

		payloadEdits = append(payloadEdits, map[string]interface{}{
			"ref":     ref,
			"content": content,
		})
	}

	files := make([]string, 0, len(touchedFiles))
	for f := range touchedFiles {
		files = append(files, f)
	}
	sort.Strings(files)

	fmt.Printf("Edits:     %d\n", len(plan.Edits))
	fmt.Printf("Files:     %d\n", len(files))

	if domApplyDryRun {
		for _, f := range files {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Printf("dry-run: OK (no files written)\n")
		return nil
	}

	fmt.Printf("Applying:  edit_elements\n")
	out, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/edit_elements",
			"",
			map[string]interface{}{
				"edits": payloadEdits,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("edit_elements failed: %w", err)
	}
	fmt.Println(out)

	if domApplyGoFmt && len(files) > 0 {
		fmt.Printf("gofmt: running\n")
		if err := runGoFmtFiles(ctx, ws, files); err != nil {
			return err
		}
		fmt.Printf("gofmt: OK\n")
	}

	if domApplyGoTest {
		// Default to testing the main CLI + internal packages (excludes cmd/tools, which may be in-flight).
		fmt.Printf("Running: go test -count=1 ./cmd/nerd/... ./internal/... (workspace)\n")
		testCmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./cmd/nerd/...", "./internal/...")
		testCmd.Dir = ws
		out, err := testCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go test failed: %w\n%s", err, string(out))
		}
		fmt.Printf("go test: PASS\n")
	}

	return nil
}

