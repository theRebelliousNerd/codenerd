package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/config"

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
