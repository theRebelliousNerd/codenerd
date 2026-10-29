package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/tools/catalog"

	"github.com/spf13/cobra"
)

// `nerd tools value` reports whether each tool earns its place.
//
// A tool earns it by doing what the harness cannot do any other way, by
// consuming far more material than it returns, or by costing far fewer tokens
// than the work it does. The ledger (.nerd/tools/.profiles) holds what every
// tool cost and returned, on every channel; the Mangle policy
// (policy/tool_value.mg) classifies each as earning, marginal, dead or unknown
// from it, against the thresholds in the tool_value section of config.json.
// The command replays the ledger into a fresh kernel and prints the kernel's
// answer. It changes nothing: a retirement proposal is a proposal, a built-in
// is only ever flagged, and a generated tool is withdrawn only through the
// work-mode approval (/tool retire in the chat).

var toolsValueJSON bool

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Inspect the tool catalog",
	Args:  cobra.NoArgs,
	RunE:  parentGroupRunE,
}

var toolsValueCmd = &cobra.Command{
	Use:   "value",
	Short: "Show whether each tool earns its place",
	Long: `Classifies every tool as earning, marginal, dead or unknown from what it
cost and returned over every session the ledger has seen: prompt share, call and
result tokens, verification, how much it consumed against how much it returned,
and how often it was used.

Nothing is retired automatically. Dead tools derive retirement proposals: a
generated tool may be proposed for removal through the work-mode approval, a
built-in is only flagged. A tool that reports no scan size has condensation
"unknown", never a guess.`,
	Example: `  nerd tools value
  nerd tools value --json`,
	Args: cobra.NoArgs,
	RunE: runToolsValue,
}

func init() {
	toolsValueCmd.Flags().BoolVar(&toolsValueJSON, "json", false, "Emit the report as JSON")
	toolsCmd.AddCommand(toolsValueCmd)
	rootCmd.AddCommand(toolsCmd)
}

func runToolsValue(cmd *cobra.Command, _ []string) error {
	root := workspaceRootOrCwd()
	userCfg, err := config.LoadUserConfig(filepath.Join(root, ".nerd", "config.json"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	registry := tools.NewRegistry()
	for _, family := range catalog.Families() {
		if err := family.Register(registry); err != nil {
			return fmt.Errorf("register %s tools: %w", family.Name, err)
		}
	}
	toolsDir := autopoiesis.DefaultConfig(root).ToolsDir
	ledger := autopoiesis.NewProfileStore(autopoiesis.ProfilesDir(toolsDir))
	snapshot := autopoiesis.ToolLedgerSnapshot{
		Ledgers: ledger.ToolLedgers(),
		Catalog: autopoiesis.CatalogOf(registry),
	}

	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		return fmt.Errorf("boot kernel: %w", err)
	}
	report, err := autopoiesis.EvaluateToolValue(kernel, snapshot, userCfg.GetToolValueConfig())
	if err != nil {
		return err
	}

	if toolsValueJSON {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), report.Table())
	return nil
}
