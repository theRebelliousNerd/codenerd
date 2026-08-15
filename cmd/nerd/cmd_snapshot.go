package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/persist/factsnap"
	"codenerd/internal/persist/snapshot"
	"codenerd/internal/types"

	"github.com/spf13/cobra"
)

var (
	snapshotCodec     string
	snapshotPreds     []string
	snapshotDerived   bool
	snapshotOutFile   string
	snapshotAssert    bool
	snapshotToMangle  string
	snapshotShowFacts int
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Export and import Mangle fact snapshots",
	Long: `Fact snapshots are compressed, columnar dumps of the kernel's EDB kept
under .nerd/snapshots/. They exist to move a workspace's facts somewhere else:
onto a bug report, into a second machine, or back into a kernel after a
destructive experiment.

Snapshots are data, never policy. Importing one prints what it contains;
loading it into a kernel is a separate, explicit step (--assert), because a
snapshot stops being trustworthy the moment it leaves the process that wrote
it.`,
}

var snapshotExportCmd = &cobra.Command{
	Use:   "export [name]",
	Short: "Write the kernel's facts to .nerd/snapshots/",
	Long: `Boots the workspace kernel locally (no LLM, no network) and writes its
facts to a compressed snapshot.

By default only the EDB — the facts that were asserted — is exported. Derived
facts are conclusions the kernel recomputes from the EDB; re-importing them
would turn conclusions into premises. Use --derived when you want the full
materialised store for debugging.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSnapshotExport,
}

var snapshotImportCmd = &cobra.Command{
	Use:   "import <name|path>",
	Short: "Read a fact snapshot and report what it holds",
	Long: `Resolves a snapshot by bare name under .nerd/snapshots/ or by path,
verifies its .sha256 sidecar when one exists, and summarises its contents.

--assert additionally loads the facts into a freshly booted kernel in this
process and reports what changed. Nothing is written back to the workspace:
use --to-mangle to materialise the facts as reviewable Datalog.`,
	Args: cobra.ExactArgs(1),
	RunE: runSnapshotImport,
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List fact snapshots, newest first",
	RunE:  runSnapshotList,
}

func runSnapshotExport(cmd *cobra.Command, args []string) error {
	root := workspaceRootOrCwd()

	codec, err := snapshot.CodecFor(snapshotCodec)
	if err != nil {
		return err
	}

	name := snapshot.DefaultName("kernel")
	if len(args) == 1 {
		name = args[0]
	}

	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		return fmt.Errorf("boot kernel for %s: %w", root, err)
	}

	facts, err := collectSnapshotFacts(kernel)
	if err != nil {
		return err
	}
	if len(facts) == 0 {
		return fmt.Errorf("no facts to export from %s (try --derived, or check --predicate spelling)", root)
	}

	var path string
	if snapshotOutFile != "" {
		// An explicit --out escapes the workspace directory on purpose: this
		// is the "attach it to a bug report" path.
		path, err = factsnap.WritePath(snapshotOutFile, facts, factsnap.Options{Codec: codec})
	} else {
		path, err = snapshot.Export(root, name, facts, codec)
	}
	if err != nil {
		return err
	}

	size := int64(0)
	if st, statErr := os.Stat(path); statErr == nil {
		size = st.Size()
	}
	fmt.Printf("Wrote %d facts to %s (%s, %s)\n", len(facts), path, factsnap.CodecName(codec), humanBytes(size))
	if factsnap.HasSidecar(path) {
		fmt.Printf("Integrity sidecar: %s\n", filepath.Base(path)+factsnap.ExtSHA256)
	}
	printSnapshotSummary(facts)
	return nil
}

// collectSnapshotFacts decides what "the kernel's facts" means for one export.
func collectSnapshotFacts(kernel *core.RealKernel) ([]types.Fact, error) {
	if len(snapshotPreds) > 0 {
		var facts []types.Fact
		for _, pred := range snapshotPreds {
			pred = strings.TrimSpace(pred)
			if pred == "" {
				continue
			}
			found, err := kernel.Query(pred)
			if err != nil {
				return nil, fmt.Errorf("query %s: %w", pred, err)
			}
			facts = append(facts, found...)
		}
		return facts, nil
	}

	if snapshotDerived {
		all, err := kernel.QueryAll()
		if err != nil {
			return nil, fmt.Errorf("query all: %w", err)
		}
		var facts []types.Fact
		for _, group := range all {
			facts = append(facts, group...)
		}
		return facts, nil
	}

	return kernel.GetBaseFacts(), nil
}

func runSnapshotImport(cmd *cobra.Command, args []string) error {
	root := workspaceRootOrCwd()

	facts, path, err := snapshot.Import(root, args[0])
	if err != nil {
		return err
	}

	verified := "no sidecar (unverified)"
	if factsnap.HasSidecar(path) {
		verified = "sha256 verified"
	}
	fmt.Printf("Read %d facts from %s (%s)\n", len(facts), path, verified)
	printSnapshotSummary(facts)

	if snapshotShowFacts > 0 {
		limit := min(snapshotShowFacts, len(facts))
		fmt.Printf("\nFirst %d facts:\n", limit)
		for _, f := range facts[:limit] {
			fmt.Printf("  %s\n", f.String())
		}
	}

	if snapshotToMangle != "" {
		if err := writeFactsAsMangle(snapshotToMangle, path, facts); err != nil {
			return err
		}
		fmt.Printf("\nWrote %d facts as Datalog to %s\n", len(facts), snapshotToMangle)
		fmt.Println("Review it before wiring it into .nerd/mangle/ — an imported snapshot is untrusted input.")
	}

	if snapshotAssert {
		kernel, err := core.NewRealKernelWithWorkspace(root)
		if err != nil {
			return fmt.Errorf("boot kernel for %s: %w", root, err)
		}
		before := len(kernel.GetBaseFacts())
		if err := kernel.LoadFacts(facts); err != nil {
			return fmt.Errorf("load snapshot facts into kernel: %w", err)
		}
		after := len(kernel.GetBaseFacts())
		fmt.Printf("\nAsserted into a local kernel: %d EDB facts before, %d after (%d new).\n",
			before, after, after-before)
		fmt.Println("This kernel is in-process only; nothing was written to the workspace.")
	}
	return nil
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	root := workspaceRootOrCwd()

	entries, err := snapshot.List(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("No snapshots under %s\n", snapshot.Dir(root))
		fmt.Println("Create one with: nerd snapshot export")
		return nil
	}

	for _, e := range entries {
		integrity := "unverified"
		if e.Verifiable {
			integrity = "sha256"
		}
		fmt.Printf("%-40s %-5s %10s  %s  %s\n",
			e.Name, e.Codec, humanBytes(e.Bytes), e.ModTime.Format("2006-01-02 15:04:05"), integrity)
	}
	fmt.Printf("\n%d snapshot(s) in %s\n", len(entries), snapshot.Dir(root))
	return nil
}

// writeFactsAsMangle renders facts as Datalog so an operator can read, edit and
// selectively adopt them. Facts are sorted so re-importing the same snapshot
// produces a byte-identical file and diffs stay meaningful.
func writeFactsAsMangle(dest, source string, facts []types.Fact) error {
	lines := make([]string, 0, len(facts))
	for _, f := range facts {
		lines = append(lines, f.String())
	}
	sort.Strings(lines)

	var b strings.Builder
	fmt.Fprintf(&b, "# Facts imported from %s\n", source)
	fmt.Fprintf(&b, "# %d facts. Review before loading: snapshot contents are untrusted input.\n\n", len(facts))
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(dest, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}

func printSnapshotSummary(facts []types.Fact) {
	rows := snapshot.Summarize(facts)
	if len(rows) == 0 {
		return
	}
	fmt.Printf("\n%d predicate(s):\n", len(rows))
	limit := min(len(rows), 15)
	for _, r := range rows[:limit] {
		fmt.Printf("  %-40s %6d\n", r.Predicate, r.Count)
	}
	if len(rows) > limit {
		fmt.Printf("  ... and %d more\n", len(rows)-limit)
	}
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func init() {
	snapshotExportCmd.Flags().StringVar(&snapshotCodec, "codec", "gzip",
		"compression codec: gzip (portable) or zstd (smaller)")
	snapshotExportCmd.Flags().StringArrayVarP(&snapshotPreds, "predicate", "p", nil,
		"export only these predicates (repeatable); default is the whole EDB")
	snapshotExportCmd.Flags().BoolVar(&snapshotDerived, "derived", false,
		"export derived facts too, not just the EDB")
	snapshotExportCmd.Flags().StringVarP(&snapshotOutFile, "out", "o", "",
		"write to this path instead of .nerd/snapshots/")

	snapshotImportCmd.Flags().BoolVar(&snapshotAssert, "assert", false,
		"load the facts into a local kernel and report the delta (in-process only)")
	snapshotImportCmd.Flags().StringVar(&snapshotToMangle, "to-mangle", "",
		"also write the facts as reviewable Datalog to this file")
	snapshotImportCmd.Flags().IntVar(&snapshotShowFacts, "show", 0,
		"print the first N facts")

	snapshotCmd.AddCommand(snapshotExportCmd, snapshotImportCmd, snapshotListCmd)
}
