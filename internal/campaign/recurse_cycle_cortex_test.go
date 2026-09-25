package campaign_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codenerd/internal/campaign"
	nerdsystem "codenerd/internal/system"
)

// The recurse loop on the production kernel: the domain shards the factory
// boots, where recurse.mg's rules fire only if their facts share a shard.
// Package campaign's own recurse tests run on a single-store RealKernel,
// which cannot show a split join.
func TestRecurseCycles_OnTheProductionKernel(t *testing.T) {
	for _, tool := range []string{"git", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":              "module example.com/m\n\ngo 1.21\n",
		".gitignore":          ".nerd/\n",
		"store/store.go":      "package store\n\nfunc Get() int { return 1 }\n",
		"store/store_test.go": "package store\n\nimport \"testing\"\n\nfunc TestGet(t *testing.T) {\n\tif Get() != 2 {\n\t\tt.Fatal(\"Get() != 2\")\n\t}\n}\n",
		"lib/lib.go":          "package lib\n\nfunc L() int { return 1 }\n",
		"lib/lib_test.go":     "package lib\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) {\n\tif L() != 0 {\n\t\tt.Fatal(\"L() != 0\")\n\t}\n}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--quiet", "-b", "main"},
		{"add", "--all"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	// store is fixable; lib's attempts do nothing, so its finding must stall
	// on the second pass and be left alone on the third.
	attempts := map[string]int{}
	res, err := campaign.RunRecurseCycles(context.Background(), campaign.RecurseCycleConfig{
		Workspace: root, Kernel: ck, Passes: 3,
		Execute: func(ctx context.Context, a campaign.RecurseAttempt) error {
			attempts[a.Node.ID]++
			if a.Node.ID == "store" {
				return os.WriteFile(filepath.Join(root, "store", "store.go"), []byte("package store\n\nfunc Get() int { return 2 }\n"), 0o644)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunRecurseCycles: %v", err)
	}
	if res.Kept != 1 || attempts["store"] != 1 {
		t.Fatalf("the store fix is picked and kept on the production kernel: attempts %v, result %+v", attempts, res)
	}
	if attempts["lib"] != 2 || len(res.Stalled) != 1 {
		t.Fatalf("lib's no-op attempts stall after two passes: attempts %v, result %+v", attempts, res)
	}
}
