package campaign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jsonl"
)

// soakGate is the gate the soak test runs as a process: it passes when
// <node>/value.txt says "ok". Built once per run, so a thousand cycles cost a
// thousand tiny process starts, not a thousand toolchain runs.
const soakGate = `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	node := os.Args[1]
	b, err := os.ReadFile(filepath.Join(node, "value.txt"))
	if got := strings.TrimSpace(string(b)); err != nil || got != "ok" {
		fmt.Printf("%s/value.txt:1: want ok, got %s\n", node, got)
		os.Exit(1)
	}
}
`

func buildSoakGate(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the soak gate")
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"go.mod": "module soakgate\n\ngo 1.21\n", "main.go": soakGate})
	bin := filepath.Join(dir, "soakgate")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build soak gate: %v\n%s", err, out)
	}
	return bin
}

// A long run leaves bounded state: however many cycles, the kernel holds at
// most two attempts per finding and one kept change per node, the in-flight
// record is cleared, and the journal grows by a few hundred bytes a cycle
// under its own cap. The default is a short soak for CI; set
// CODENERD_RECURSE_SOAK=1000 (or any count) for a long one.
func TestRecurseCycles_ALongRunLeavesBoundedState(t *testing.T) {
	cycles := 100
	if v := os.Getenv("CODENERD_RECURSE_SOAK"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("CODENERD_RECURSE_SOAK=%q: %v", v, err)
		}
		cycles = n
	}
	bin := buildSoakGate(t)
	const nodes = 6
	files := map[string]string{
		".gitignore": ".nerd/\n",
		// A folded scalar takes the command verbatim; single quotes keep the
		// binary's path one word for the gate's shell-style split.
		"nerd.md": "---\nschema: nerd/v1\ngates:\n  - id: check\n    kind: test\n    run: >-\n      '" + filepath.ToSlash(bin) + "' {node}\n    scope: node\n---\n",
	}
	for i := range nodes {
		dir := fmt.Sprintf("n%d", i)
		files[dir+"/__init__.py"] = ""
		files[dir+"/value.txt"] = "bad\n"
	}
	root := gitFixture(t, files).root

	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A fake model that never lets the workspace go green: a fix is kept but
	// breaks the next node; an attempt does nothing; an attempt changes its
	// node but fails differently every time. Improvements move nothing.
	res, err := RunRecurseCycles(ctx, RecurseCycleConfig{
		Workspace: root, Kernel: k,
		Execute: func(ctx context.Context, a RecurseAttempt) error {
			if a.Cycle >= cycles {
				cancel()
				return nil
			}
			if a.Angle != "" {
				return nil
			}
			node := a.Node.Paths[0]
			switch a.Cycle % 3 {
			case 0:
				var n int
				fmt.Sscanf(node, "n%d", &n)
				next := fmt.Sprintf("n%d", (n+1)%nodes)
				write(t, root, node+"/value.txt", "ok\n")
				write(t, root, next+"/value.txt", "bad-"+strconv.Itoa(a.Cycle)+"\n")
			case 1:
			case 2:
				write(t, root, node+"/value.txt", "wrong "+strings.Repeat("x", a.Cycle%17+1)+"\n")
			}
			return nil
		},
	})
	if err != nil && ctx.Err() == nil {
		t.Fatalf("RunRecurseCycles: %v", err)
	}
	if res.Cycles < cycles {
		t.Fatalf("ran %d cycles, want %d: %+v", res.Cycles, cycles, res)
	}

	count := func(pred string) int {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	// Each node has one finding at a time (its value), but its identity
	// changes with the value's message; per node, at most a handful of
	// distinct findings are ever attempted between two kept changes.
	if got := count("recurse_attempt"); got > 2*nodes*3 {
		t.Fatalf("%d cycles left %d recurse_attempt facts; the kernel's recurse memory is not bounded", res.Cycles, got)
	}
	if got := count("recurse_node_kept"); got > nodes {
		t.Fatalf("%d recurse_node_kept facts for %d nodes", got, nodes)
	}
	if got := count("recurse_visit_attempted"); got > nodes*3 {
		t.Fatalf("%d recurse_visit_attempted facts", got)
	}
	for pred := range ratchetInputs {
		if got := count(pred); got != 0 {
			t.Fatalf("%d %s facts outlived their cycle", got, pred)
		}
	}
	if _, err := os.Stat(inFlightPath(root)); !os.IsNotExist(err) {
		t.Fatalf("the in-flight record outlived the run: %v", err)
	}
	var size int64
	for _, p := range []string{RecurseJournalPath(root), RecurseJournalPath(root) + ".1"} {
		if st, err := os.Stat(p); err == nil {
			size += st.Size()
		}
	}
	if size > 2*jsonl.DefaultMaxBytes {
		t.Fatalf("journal %d bytes exceeds its cap", size)
	}
	t.Logf("%d cycles (%d kept, %d reverted): journal %d bytes (%d per cycle), %d attempt facts",
		res.Cycles, res.Kept, res.Reverted, size, size/int64(res.Cycles), count("recurse_attempt"))
}

// An attempt's campaign leaves nothing behind once released: not its files,
// not its facts; another campaign's are untouched.
func TestReleaseRecurseAttempt_DropsTheAttemptsCampaign(t *testing.T) {
	ws := t.TempDir()
	c := RecurseAttemptCampaign(ws, RecurseAttempt{Cycle: 1, Node: SubsystemNode{ID: "store", Title: "store", Paths: []string{"store"}}, Angle: "stabilize"})
	other := RecurseAttemptCampaign(ws, RecurseAttempt{Cycle: 2, Node: SubsystemNode{ID: "web", Title: "web", Paths: []string{"web"}}, Angle: "stabilize"})
	for _, camp := range []*Campaign{c, other} {
		slug := sanitizeCampaignID(camp.ID)
		writeTree(t, ws, map[string]string{
			".nerd/campaigns/" + slug + ".json":          "{}",
			".nerd/campaigns/" + slug + ".journal.jsonl": "{}\n",
			".nerd/campaigns/" + slug + "/knowledge.db":  "db",
		})
	}
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := k.LoadFacts(append(c.ToFacts(), other.ToFacts()...)); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseRecurseAttempt(ws, k, c); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(ws, ".nerd", "campaigns", "*"))
	for _, p := range left {
		if strings.Contains(p, sanitizeCampaignID(c.ID)) {
			t.Fatalf("released campaign file remains: %s", p)
		}
	}
	if len(left) != 3 {
		t.Fatalf("the other campaign's files must stay: %v", left)
	}
	rows, err := k.Query("campaign")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if len(r.Args) > 0 && fmt.Sprint(r.Args[0]) == c.ID {
			t.Fatalf("released campaign's facts remain: %v", r)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("the other campaign's facts must stay: %v", rows)
	}
}
