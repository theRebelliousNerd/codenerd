package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"

	"github.com/spf13/cobra"
)

// `nerd audit` is the operator surface for the durable record internal/logging
// writes. Two things were missing: the audit JSONL carried a pre-formatted
// Mangle fact on every line that nothing could turn into a loadable .mg file,
// and the only documentation of how to switch diagnostics on lived in a corpus
// directory nobody finds from the CLI.

var (
	auditFactsOut    string
	auditFactsLog    string
	auditFactsEvents []string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect the codeNERD audit trail",
	Long: "Reads .nerd/logs/<run>_audit.log — the JSONL record of shard, action, tool,\n" +
		"kernel, LLM and safety events — and exports or explains it.",
}

var auditFactsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Export the audit log as a Mangle facts (.mg) file",
	Long: "Converts the audit JSONL into a standalone .mg file: Decl statements for every\n" +
		"predicate present plus the deduplicated facts. The result is an OFFLINE forensic\n" +
		"artifact — it is not loaded into the live kernel, because telemetry that feeds the\n" +
		"executive would let the record of what happened change what happens next.",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := auditFactsLog
		if path == "" {
			var err error
			path, err = logging.LatestAuditLogPath()
			if err != nil {
				return auditLogHint(err)
			}
		}

		events := make([]logging.AuditEventType, 0, len(auditFactsEvents))
		for _, e := range auditFactsEvents {
			e = strings.TrimSpace(e)
			if e != "" {
				events = append(events, logging.AuditEventType(e))
			}
		}

		out := os.Stdout
		if auditFactsOut != "" {
			if dir := filepath.Dir(auditFactsOut); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
			}
			f, err := os.Create(auditFactsOut)
			if err != nil {
				return fmt.Errorf("creating %s: %w", auditFactsOut, err)
			}
			defer f.Close()
			out = f
		}

		stats, err := logging.ExportAuditFacts(path, out, events)
		if err != nil {
			return auditLogHint(err)
		}

		if auditFactsOut != "" {
			names := make([]string, 0, len(stats.Predicates))
			for name := range stats.Predicates {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Printf("Wrote %s\n", auditFactsOut)
			fmt.Printf("  source:     %s\n", path)
			fmt.Printf("  events:     %d\n", stats.Events)
			fmt.Printf("  facts:      %d (%d duplicates collapsed)\n", stats.Facts, stats.Duplicates)
			fmt.Printf("  predicates: %s\n", strings.Join(names, ", "))
		}
		return nil
	},
}

var auditPlaybookCmd = &cobra.Command{
	Use:   "playbook",
	Short: "How to turn on diagnostics and where the logs land",
	Long:  "Prints the operator playbook for the logging subsystem.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(loggingPlaybook)
	},
}

// auditLogHint turns "no audit log" into the actionable version. Audit logging
// is gated on debug_mode, so its absence almost always means diagnostics are
// off — reporting "nothing recorded" would state a fact about the system when
// the fact is about its configuration.
func auditLogHint(err error) error {
	if err == logging.ErrNoAuditLog {
		return fmt.Errorf("%w\n\nEnable it with .nerd/config.json:\n%s", err, minimalLoggingConfig)
	}
	return err
}

const minimalLoggingConfig = `  {
    "logging": {
      "debug_mode": true,
      "level": "debug",
      "format": "text"
    }
  }
`

const loggingPlaybook = `codeNERD logging — operator playbook
====================================

Master switch (.nerd/config.json, the same file every other subsystem reads):

  {
    "logging": {
      "debug_mode": true,               // false = silent production, no files written
      "level": "debug",                 // debug | info | warn | error
      "format": "text",                 // text | json  ("json_format": true is a legacy alias)
      "trace_llm_io": false,            // full prompt/response dump
      "trace_llm_io_raw": false,        // disable secret redaction in that dump (unsafe)
      "categories": { "kernel": true, "session": true },
      "performance_sampling": 0.1,
      "performance_thresholds_ms": { "default": 100, "kernel": 50 },
      "max_log_file_mb": 32,            // rotate a segment past this size (-1 = never)
      "max_log_file_minutes": 0,        // rotate a segment older than this (0 = off)
      "max_rotated_files": 3            // archived segments kept per file
    }
  }

Where it lands (.nerd/logs/, one set per run, newest 10 runs kept):

  <run>_boot.log        startup sequence
  <run>_problems.log    every WARN and ERROR from every category, interleaved — start here
  <run>_audit.log       JSONL event record, one Mangle fact per line
  <run>_llm_io.log      full prompt packages (only when trace_llm_io is on)
  <run>_<category>.log  one file per category

Commands:

  nerd logs                        errors and warnings grouped by category
  nerd audit facts --out a.mg      audit JSONL -> loadable Mangle facts (offline)
  nerd audit facts --event safety_block --event safety_allow
  nerd transparency                live decision surface (different subsystem)

Triage order that actually works:

  1. <run>_problems.log — one file, every failure, in time order.
  2. nerd audit facts, filtered to the event family you suspect.
  3. <run>_llm_io.log if the failure is prompt-shaped. Redaction is on by
     default; only set trace_llm_io_raw when a masked value IS the bug, and
     delete the file afterwards.

Reference: Docs/architecture/logging/ (README, IMPLEMENTED_SPEC, 09-SAFETY-AND-INVARIANTS).
This is NOT the zap console logger, NOT transparency/glass-box, and NOT
internal/observability metrics — those are adjacent surfaces with their own docs.
`

func init() {
	auditFactsCmd.Flags().StringVarP(&auditFactsOut, "out", "o", "", "Write facts to this .mg file (default: stdout)")
	auditFactsCmd.Flags().StringVar(&auditFactsLog, "log", "", "Audit log to read (default: newest in .nerd/logs)")
	auditFactsCmd.Flags().StringSliceVar(&auditFactsEvents, "event", nil, "Only export these event types (repeatable)")
	auditCmd.AddCommand(auditFactsCmd, auditPlaybookCmd)
}
