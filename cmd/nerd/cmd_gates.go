package main

// nerd gates: the ladder's error-free gates (G1 build .. G10 gofmt) measured
// as data. `measure` writes one typed result per gate to a snapshot under
// .nerd/gates/; `diff` asks the kernel (policy/gates.mg) how two snapshots
// compare -- per gate better, worse or same, the findings that are new and
// gone -- and exits 1 when a gate got worse, which is rung R7's keep rule.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/gates"
	"codenerd/internal/lintreport"
	"codenerd/internal/prompt"

	"github.com/spf13/cobra"
)

var (
	gatesOnly     []string
	gatesTests    []string
	gatesFindings bool
)

var gatesCmd = &cobra.Command{
	Use:   "gates",
	Short: "Measure the error-free gates as data, and compare two measurements",
}

var gatesMeasureCmd = &cobra.Command{
	Use:   "measure",
	Short: "Measure the error-free gates into a snapshot under .nerd/gates/",
	Long: `Measures the ladder's error-free gates over the workspace, each with the
command its table names: G1 go build, G2 go vet, G3 go test (--tests narrows it),
G4 staticcheck, G5 golangci-lint (no issue caps), G6 govulncheck, G10 gofmt, and on
codeNERD's own tree G7 the policy corpus (check-mangle) and G8 the atom corpus's
Mangle examples. A gate whose program is not installed is reported unavailable,
never passed. Each gate's findings carry file, line, check and message.`,
	Example: `  nerd gates measure
  nerd gates measure --only vet,staticcheck,gofmt
  nerd gates measure --tests ./internal/campaign/...`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		root := workspaceRootOrCwd()
		opts := codeNERDGateOptions(gatesOnly, gatesTests)
		if unknown := gates.UnknownGates(opts); len(unknown) > 0 {
			return fmt.Errorf("--only names no gate: %s", strings.Join(unknown, ", "))
		}
		snap := gates.MeasureErrorFree(cmd.Context(), root, opts)
		path, err := snap.Save(root)
		if err != nil {
			return err
		}
		printSnapshot(cmd.OutOrStdout(), snap)
		fmt.Fprintf(cmd.OutOrStdout(), "\nsnapshot: %s\n", path)
		return nil
	},
}

var gatesDiffCmd = &cobra.Command{
	Use:   "diff [BEFORE AFTER]",
	Short: "Compare two gate snapshots; exit 1 when a gate got worse",
	Long: `Compares two snapshots (paths, or the two most recent under .nerd/gates/
when none are given). The verdict is the kernel's (policy/gates.mg): a gate is
worse when a pass became a fail, a measured gate could no longer be measured, a
failing gate's findings grew, or named findings became a failure that names none.
A gate measured in one snapshot only, or over another workspace or package
selection (--tests), is not compared, and the pair is not keepable. The command
exits 1 when the pair is not keepable.`,
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 0 && len(args) != 2 {
			return fmt.Errorf("diff takes two snapshots or none")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		before, after, err := snapshotPair(workspaceRootOrCwd(), args)
		if err != nil {
			return err
		}
		k, err := core.NewRealKernel()
		if err != nil {
			return fmt.Errorf("boot the kernel to judge the snapshots: %w", err)
		}
		c, err := gates.Compare(k, before, after)
		if err != nil {
			return err
		}
		printComparison(cmd.OutOrStdout(), c, gatesFindings)
		if !c.Keepable {
			cmd.SilenceUsage = true
			return fmt.Errorf("not keepable: %s", strings.Join(c.NotKeepableBecause(), "; "))
		}
		return nil
	},
}

func init() {
	gatesMeasureCmd.Flags().StringSliceVar(&gatesOnly, "only", nil, "measure only these gates (names or labels: vet,G4,...)")
	gatesMeasureCmd.Flags().StringSliceVar(&gatesTests, "tests", nil, "package patterns G3 runs (default ./...)")
	gatesDiffCmd.Flags().BoolVar(&gatesFindings, "findings", false, "list every new and gone finding")
	gatesCmd.AddCommand(gatesMeasureCmd, gatesDiffCmd)
	rootCmd.AddCommand(gatesCmd)
}

// codeNERDGateOptions adds the gates only codeNERD's own tree has: G7 and G8.
func codeNERDGateOptions(only, tests []string) gates.MeasureOptions {
	return gates.MeasureOptions{Only: only, TestPackages: tests, Extra: []gates.Measurer{policyCorpusGate{}, atomCorpusGate{}}}
}

func snapshotPair(root string, args []string) (gates.Snapshot, gates.Snapshot, error) {
	paths := args
	if len(paths) == 0 {
		all, err := gates.SnapshotPaths(root)
		if err != nil {
			return gates.Snapshot{}, gates.Snapshot{}, err
		}
		if len(all) < 2 {
			return gates.Snapshot{}, gates.Snapshot{}, fmt.Errorf("%s holds %d snapshot(s); measure twice, or name two", gates.SnapshotDir, len(all))
		}
		paths = all[len(all)-2:]
	}
	before, err := gates.LoadSnapshot(paths[0])
	if err != nil {
		return gates.Snapshot{}, gates.Snapshot{}, err
	}
	after, err := gates.LoadSnapshot(paths[1])
	return before, after, err
}

func printSnapshot(w io.Writer, s gates.Snapshot) {
	fmt.Fprintf(w, "gates %s (head %s)\n", s.ID, orNone(s.Head))
	for _, g := range s.Gates {
		line := fmt.Sprintf("  %-4s %-14s %-11s %6d", g.Label, g.Gate, g.Status, g.Count)
		if g.Reason != "" && g.Status != gates.MeasurePass {
			line += "  " + g.Reason
		}
		fmt.Fprintln(w, line)
	}
}

func printComparison(w io.Writer, c gates.Comparison, findings bool) {
	verdict := "keepable: every gate compared, none got worse"
	if !c.Keepable {
		verdict = "NOT keepable: " + strings.Join(c.NotKeepableBecause(), "; ")
	}
	fmt.Fprintf(w, "%s -> %s: %s\n", c.Before, c.After, verdict)
	for _, g := range c.Gates {
		fmt.Fprintf(w, "  %-4s %-14s %-6s %s %d -> %s %d  (+%d new, -%d gone)\n",
			g.Label, g.Gate, g.Change, g.Before, g.BeforeCount, g.After, g.AfterCount, len(g.New), len(g.Gone))
		if findings {
			for _, k := range g.New {
				fmt.Fprintf(w, "      + %s\n", k)
			}
			for _, k := range g.Gone {
				fmt.Fprintf(w, "      - %s\n", k)
			}
		}
	}
	for _, u := range c.Unadmitted {
		fmt.Fprintf(w, "  %-4s %-14s not compared: %s\n", u.Label, u.Gate, strings.ReplaceAll(u.Reason, "_", " "))
	}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// policyCorpusGate is G7: every policy, schema and default .mg file loads on
// the pinned engine with codeNERD's shared schemas preloaded -- `nerd
// check-mangle` over the table's three globs, run in-process.
type policyCorpusGate struct{}

func (policyCorpusGate) Gate() string  { return "policy_corpus" }
func (policyCorpusGate) Label() string { return "G7" }

var policyCorpusGlobs = []string{"internal/core/defaults/*.mg", "internal/core/defaults/policy/*.mg", "internal/core/defaults/schema/*.mg"}

func (policyCorpusGate) Measure(_ context.Context, root string) gates.Measurement {
	m := gates.Measurement{Argv: append([]string{"nerd", "check-mangle"}, policyCorpusGlobs...)}
	if !dirExists(filepath.Join(root, "internal", "core", "defaults", "policy")) {
		m.Status, m.Reason = gates.MeasureUnavailable, "not codeNERD's tree: no internal/core/defaults/policy"
		return m
	}
	start := time.Now()
	var files []string
	for _, g := range policyCorpusGlobs {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(g)))
		if err != nil {
			m.Status, m.Reason = gates.MeasureUnverified, err.Error()
			return m
		}
		files = append(files, matches...)
	}
	for _, f := range files {
		if _, err := checkFile(io.Discard, f, false); err != nil {
			m.Findings = append(m.Findings, lintreport.Finding{File: lintreport.Path(f, root), Check: "check-mangle", Message: err.Error()})
		}
	}
	m.Duration = time.Since(start)
	m.Status = gates.MeasurePass
	if len(m.Findings) > 0 {
		m.Status = gates.MeasureFail
	}
	m.Reason = fmt.Sprintf("%d file(s) checked", len(files))
	return m
}

// atomCorpusGate is G8: the Mangle examples in the prompt atoms on disk that
// fail to load unmarked (prompt.CountMangleExamples, the measurement
// TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow ratchets). A finding
// is an atom's block; the engine's error is in Output.
type atomCorpusGate struct{}

func (atomCorpusGate) Gate() string  { return "atom_corpus" }
func (atomCorpusGate) Label() string { return "G8" }

func (atomCorpusGate) Measure(_ context.Context, root string) gates.Measurement {
	m := gates.Measurement{}
	dir := filepath.Join(root, "internal", "prompt", "atoms")
	if !dirExists(dir) {
		m.Status, m.Reason = gates.MeasureUnavailable, "not codeNERD's tree: no internal/prompt/atoms"
		return m
	}
	start := time.Now()
	parsed, _, err := prompt.ParsePromptAtomDirectory(dir)
	if err != nil {
		m.Status, m.Reason = gates.MeasureUnverified, "parse the atoms: "+err.Error()
		return m
	}
	atoms := make([]*prompt.PromptAtom, 0, len(parsed))
	source := map[string]string{}
	for _, p := range parsed {
		atoms = append(atoms, p.Atom)
		if p.Atom != nil {
			source[p.Atom.ID] = lintreport.Path(p.SourcePath, root)
		}
	}
	count, err := prompt.CountMangleExamples(atoms)
	if err != nil {
		m.Status, m.Reason = gates.MeasureUnverified, err.Error()
		return m
	}
	var out strings.Builder
	for _, ex := range count.Failing {
		m.Findings = append(m.Findings, lintreport.Finding{File: source[ex.AtomID], Check: "mangle-example", Message: fmt.Sprintf("%s block %d", ex.AtomID, ex.Block)})
		fmt.Fprintf(&out, "%s block %d: %s\n", ex.AtomID, ex.Block, ex.Error)
	}
	m.Duration = time.Since(start)
	m.Status = gates.MeasurePass
	if len(m.Findings) > 0 {
		m.Status, m.Output = gates.MeasureFail, strings.TrimSpace(out.String())
	}
	m.Reason = fmt.Sprintf("%d Mangle block(s), %d load", count.Total, count.Loading)
	return m
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
