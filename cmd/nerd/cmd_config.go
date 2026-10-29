package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/system"

	"github.com/spf13/cobra"
)

// configCmd inspects .nerd/config.json. It is the one command that runs on a
// config the loader refuses: a file you cannot load is the file you need to
// read about.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Check .nerd/config.json, or print it with every field",
}

var configCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "List every error, warning and defaulted field in .nerd/config.json",
	Long: `Reads .nerd/config.json and reports, by JSON path:

  error     settings that contradict each other or fall outside their vocabulary
            (a model its provider cannot serve, an unknown engine). codeNERD
            refuses to start on any of these.
  warning   something a run will need and the file does not say (no model, no
            key for the named provider, no embedding model).
  implicit  every configurable field the file does not mention, so a default
            is in force. --no-implicit hides these.

Exits 1 when there is an error, 0 otherwise.`,
	RunE: runConfigCheck,
}

var configFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Print the config with every configurable field and the value in force",
	Long: `Prints .nerd/config.json laid over the defaults, with every field written
down -- including false, 0 and "" -- in the schema's order. The output is a
loadable config meant to be merged into config.json by hand; this command never
writes config.json. It contains your API keys, as the file does: use --out to
write it to a file rather than a terminal.`,
	RunE: runConfigFull,
}

var (
	configNoImplicit bool
	configFullOut    string
)

func init() {
	configCheckCmd.Flags().BoolVar(&configNoImplicit, "no-implicit", false, "Hide the defaulted fields; show errors and warnings only")
	configFullCmd.Flags().StringVar(&configFullOut, "out", "", "Write the document to this file instead of stdout (never .nerd/config.json)")
	configCmd.AddCommand(configCheckCmd, configFullCmd)
}

func userConfigPath() string {
	if workspace != "" {
		return filepath.Join(workspace, ".nerd", "config.json")
	}
	return config.DefaultUserConfigPath()
}

func runConfigCheck(cmd *cobra.Command, _ []string) error {
	path := userConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	cfg, err := config.DecodeUserConfig(raw)
	if err != nil {
		// Not even decodable: an unknown or removed key, or broken JSON.
		return fmt.Errorf("%s does not decode: %w", path, err)
	}
	counts := map[config.Severity]int{}
	for _, p := range cfg.Check(raw) {
		counts[p.Severity]++
		if p.Severity == config.SeverityImplicit && configNoImplicit {
			continue
		}
		fmt.Fprintln(cmd.OutOrStdout(), p.String())
	}
	// Boot refuses any user agent whose tools name a tool the host has not
	// registered (a stderr warning, not a boot error), so the file that
	// "passes" this command can still lose an agent at startup. The sibling
	// agents.json is checked with boot's own test and reported the same way.
	for _, p := range checkUserAgentTools() {
		counts[p.Severity]++
		fmt.Fprintln(cmd.OutOrStdout(), p.String())
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %d error(s), %d warning(s), %d field(s) left to defaults\n",
		path, counts[config.SeverityError], counts[config.SeverityWarning], counts[config.SeverityImplicit])
	if counts[config.SeverityError] > 0 {
		return fmt.Errorf("%d error(s): codeNERD will not start on this file", counts[config.SeverityError])
	}
	return nil
}

func runConfigFull(cmd *cobra.Command, _ []string) error {
	path := userConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	full, err := config.FullJSON(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if configFullOut == "" {
		_, err = cmd.OutOrStdout().Write(full)
		return err
	}
	out, err := filepath.Abs(configFullOut)
	if err != nil {
		return err
	}
	if live, _ := filepath.Abs(path); out == live {
		return fmt.Errorf("refusing to write %s: merge the document into it by hand", path)
	}
	if err := os.WriteFile(out, full, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes; contains your API keys)\n", out, len(full))
	return nil
}

// userAgentsWorkspace is the workspace whose .nerd/agents.json siblings the
// config being checked. It mirrors userConfigPath: --workspace when given,
// otherwise the root FindWorkspaceRoot resolves for the default config path.
func userAgentsWorkspace() string {
	if workspace != "" {
		return workspace
	}
	if root, err := config.FindWorkspaceRoot(); err == nil {
		return root
	}
	return "."
}

// checkUserAgentTools reports every sibling agents.json tool boot would
// refuse, as warning Problems in the same shape as cfg.Check. The loader is
// boot's (system.LoadUserAgentDefinitions) and the test is boot's
// (system.RefusedUserAgentTools over a kernel hydrated with the same
// tool_registered facts), so this command and boot refuse the same entries.
//
// A kernel that cannot be built or queried is a warning, not an error: the
// config file itself is not what failed, and this command exits 1 only for
// file errors. No agents.json, or an unreadable one, yields nothing, exactly
// as boot's best-effort load does.
func checkUserAgentTools() []config.Problem {
	ws := userAgentsWorkspace()
	defs := system.LoadUserAgentDefinitions(ws)
	if len(defs) == 0 {
		return nil
	}
	kernel, err := system.NewDomainCortex(ws)
	if err != nil {
		return []config.Problem{{
			Severity: config.SeverityWarning,
			Path:     "agents",
			Message:  fmt.Sprintf("could not verify user agent tools: %v", err),
			Fix:      "run again; if it persists, boot cannot build its kernel either",
		}}
	}
	hydrateConfigCheckToolFacts(kernel, ws)
	refusals, err := system.RefusedUserAgentTools(kernel, defs)
	if err != nil {
		return []config.Problem{{
			Severity: config.SeverityWarning,
			Path:     "agents",
			Message:  fmt.Sprintf("could not verify user agent tools: %v", err),
			Fix:      "run again; if it persists, boot cannot query its kernel either",
		}}
	}
	out := make([]config.Problem, 0, len(refusals))
	for _, r := range refusals {
		out = append(out, config.Problem{
			Severity: config.SeverityWarning,
			Path:     fmt.Sprintf("agents.%s.tools", r.Agent),
			Message:  fmt.Sprintf("user agent %q declares tool %q, which is not a registered tool; boot refuses this agent", r.Agent, r.Tool),
			Fix:      fmt.Sprintf("remove %q from the agent's tools in .nerd/agents.json, or register a tool by that name", r.Tool),
		})
	}
	return out
}

// hydrateConfigCheckToolFacts replays the tool_registered facts boot asserts
// before it registers user agents (initExecutionLayer), so the refusal test
// sees the same registry boot does. Static tools come from
// .nerd/tools/available_tools.json, compiled tools from
// .nerd/tools/.compiled; both restores batch their facts into one evaluation
// each. The Ouroboros executor sync boot's HydrateToolsFromDisk would also
// attempt is skipped there too — the executor is set only later, in
// initAutopoiesisAndBrowser — so a check without it matches boot exactly.
// Every failure here is best-effort with boot's warning, for boot's reason:
// a corrupt tools file must not stop the listing, and the agents it strands
// are reported as refused, which is what boot does with the file missing.
func hydrateConfigCheckToolFacts(kernel core.Kernel, ws string) {
	nerdDir := filepath.Join(ws, ".nerd")
	registry := core.NewToolRegistry(ws)
	registry.SetKernel(kernel)
	if static, err := system.WorkspaceStaticToolDefs(nerdDir); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to load available_tools.json: %v", err)
	} else if len(static) > 0 {
		if err := registry.RestoreFromStaticDefs(static); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to hydrate static tools: %v", err)
		}
	}
	if err := registry.RestoreFromDisk(filepath.Join(nerdDir, "tools", ".compiled")); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to hydrate tools from disk: %v", err)
	}
}

// invokesConfigCommand reports whether the command line's first non-flag word
// is "config". Flags that take a value (-w/--workspace) are skipped with it.
func invokesConfigCommand(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-w" || arg == "--workspace":
			i++
		case len(arg) > 0 && arg[0] == '-':
		default:
			return arg == "config"
		}
	}
	return false
}
