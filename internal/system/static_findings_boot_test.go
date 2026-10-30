package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The static-analysis findings (reviewer.mg raw_finding) were derived and read
// by nothing. The world scan now measures security sinks, complexity and file
// length (world/review_metrics.go), and the commit gate reads the verdict: a
// critical finding, or a risk over budget, in a file the working tree modified
// blocks the commit. This boots the real cortex on a workspace whose files carry
// the findings and watches block_commit change.
func TestStaticFindingsReachTheCommitGateOnTheShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/app\n\ngo 1.26\n")
	// SQL built from a parameter: a critical SEC001 finding.
	write("store/db.go", "package store\n\nimport \"database/sql\"\n\nfunc Find(db *sql.DB, name string) {\n\tdb.Query(\"SELECT * FROM t WHERE n = '\" + name + \"'\")\n}\n")
	// The same flaw in a file nobody touches.
	write("legacy/old.go", "package legacy\n\nimport \"database/sql\"\n\nfunc Find(db *sql.DB, name string) {\n\tdb.Query(\"SELECT * FROM t WHERE n = '\" + name + \"'\")\n}\n")
	// Three weak-crypto findings (SEC006, a warning) in three clean-looking files.
	for i := 1; i <= 3; i++ {
		write(fmt.Sprintf("hash/h%d.go", i), fmt.Sprintf("package hash\n\nimport \"crypto/md5\"\n\nfunc Sum%d(b []byte) [16]byte {\n\treturn md5.Sum(b)\n}\n", i))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	findings, err := cortex.Kernel.Query("raw_finding")
	if err != nil {
		t.Fatal(err)
	}
	if !findingFor(findings, "store/db.go", "SEC001") || !findingFor(findings, "legacy/old.go", "SEC001") {
		t.Fatalf("the boot scan must measure the SQL flow and the reviewer rules must derive SEC001: %v", findings)
	}
	if !findingFor(findings, "hash/h1.go", "SEC006") {
		t.Fatalf("weak crypto must derive SEC006: %v", findings)
	}

	// Nothing modified yet: the gate judges the change, not the repository.
	blocks, err := cortex.Kernel.Query("block_commit")
	if err != nil {
		t.Fatal(err)
	}
	if factWith(blocks, "critical_static_finding") {
		t.Fatalf("a finding in files nobody modified must not block a commit: %v", blocks)
	}

	// Modify the clean-looking file and the flawed one.
	if err := cortex.Kernel.AssertBatch([]types.Fact{
		{Predicate: "modified", Args: []any{"store/db.go"}},
		{Predicate: "modified", Args: []any{"hash/h1.go"}},
	}); err != nil {
		t.Fatal(err)
	}
	blocks, err = cortex.Kernel.Query("block_commit")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(blocks, "critical_static_finding") {
		t.Errorf("a critical finding in a modified file must block the commit: %v", blocks)
	}
	risk, err := cortex.Kernel.Query("file_finding_risk")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(risk, "store/db.go", "100") || !factWith(risk, "hash/h1.go", "20") {
		t.Errorf("risk is the sum of rule weights per file (db 100, md5 20): %v", risk)
	}

	// The user rejects the weak-crypto findings three times: the rule is
	// learned to be wrong here and stops producing active findings.
	var rejections []types.Fact
	for i := 1; i <= 3; i++ {
		rejections = append(rejections, types.Fact{Predicate: "user_rejected_finding",
			Args: []any{"rev1", fmt.Sprintf("hash/h%d.go", i), int64(6), "not a secret", int64(i)}})
	}
	if err := cortex.Kernel.AssertBatch(rejections); err != nil {
		t.Fatal(err)
	}
	active, err := cortex.Kernel.Query("active_finding")
	if err != nil {
		t.Fatal(err)
	}
	if findingFor(active, "hash/h1.go", "SEC006") {
		t.Errorf("a rule rejected three times must stop producing active findings: %v", active)
	}
	if !findingFor(active, "store/db.go", "SEC001") {
		t.Errorf("other rules keep producing: %v", active)
	}
}

// findingFor matches a raw/active finding by file (arg 0) and rule (arg 4).
func findingFor(rows []types.Fact, file, rule string) bool {
	for _, f := range rows {
		if len(f.Args) >= 5 && types.ExtractString(f.Args[0]) == file && types.ExtractString(f.Args[4]) == rule {
			return true
		}
	}
	return false
}
