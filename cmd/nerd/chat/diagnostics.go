package chat

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/observability"
	"codenerd/internal/sqlpragmas"

	tea "github.com/charmbracelet/bubbletea"
)

// renderDiagnostics is the "### Diagnostics" block of /status: the process
// state the logging, SQLite and trace subsystems keep in memory and that no
// file on disk reports while the session is still running.
//
// Each line answers a question an operator otherwise has to reconstruct from
// logs -- are my logs landing in this workspace, is the LLM trace on, how much
// of the performance log did sampling drop, did a SQLite pragma fail, can a
// trace be dumped -- and each was an exported reader with no caller.
func renderDiagnostics(workspace string) string {
	var sb strings.Builder
	sb.WriteString("\n### Diagnostics\n")

	bound := logging.BoundWorkspace()
	switch {
	case bound == "":
		sb.WriteString("- Logs: not initialized\n")
	case workspace != "" && !sameWorkspacePath(bound, workspace):
		sb.WriteString(fmt.Sprintf("- Logs: bound to %s -- NOT this workspace (%s); this session's log lines are going there\n", bound, workspace))
	default:
		sb.WriteString(fmt.Sprintf("- Logs: %s\n", filepath.Join(bound, ".nerd", "logs")))
	}

	if logging.IsLLMIOTracingEnabled() {
		sb.WriteString("- LLM I/O trace: on (logging.trace_llm_io)\n")
	} else {
		sb.WriteString("- LLM I/O trace: off\n")
	}

	if seen, dropped := logging.PerformanceSamplingStats(); dropped > 0 {
		sb.WriteString(fmt.Sprintf("- Performance log: sampling dropped %d of %d non-slow timings so far (slow operations are always logged)\n", dropped, seen))
	} else if seen > 0 {
		sb.WriteString(fmt.Sprintf("- Performance log: %d non-slow timings, none dropped by sampling\n", seen))
	}

	sb.WriteString(fmt.Sprintf("- SQLite host class: %s\n", sqlpragmas.ActiveHostClass()))
	if !sqlpragmas.MetricsEnabled() {
		sb.WriteString(fmt.Sprintf("- SQLite pragma failures: not recorded (on with logging.debug_mode or %s=1)\n", sqlpragmas.EnvMetrics))
	} else if total := sqlpragmas.PragmaFailureTotal(); total == 0 {
		sb.WriteString("- SQLite pragma failures: none\n")
	} else {
		byStatement := sqlpragmas.PragmaFailuresByStatement()
		rejected := make([]string, 0, len(byStatement))
		for _, stmt := range sqlpragmas.FailingPragmas() {
			rejected = append(rejected, fmt.Sprintf("%s (x%d)", stmt, byStatement[stmt]))
		}
		sb.WriteString(fmt.Sprintf("- SQLite pragma failures: %d -- the driver rejected: %s\n",
			total, strings.Join(rejected, "; ")))
		byProfile := sqlpragmas.PragmaFailuresByProfile()
		profiles := make([]string, 0, len(byProfile))
		for p := range byProfile {
			profiles = append(profiles, p)
		}
		sort.Strings(profiles)
		for _, p := range profiles {
			sb.WriteString(fmt.Sprintf("  - %s: %d\n", p, byProfile[p]))
		}
	}

	if observability.FlightRecorderEnabled() {
		sb.WriteString("- Flight recorder: on -- `/flightrec` dumps the trace window to .nerd/traces/\n")
	} else {
		sb.WriteString("- Flight recorder: off (features.flight_recorder or CODENERD_FLIGHT_RECORDER=1 at start)\n")
	}
	return sb.String()
}

// sameWorkspacePath compares two workspace paths the way logging binds them:
// absolute and cleaned.
func sameWorkspacePath(a, b string) bool {
	abs := func(p string) string {
		if resolved, err := filepath.Abs(p); err == nil {
			return filepath.Clean(resolved)
		}
		return filepath.Clean(p)
	}
	return abs(a) == abs(b)
}

// handleCmdFlightrec dumps the flight recorder's window on demand.
//
// The recorder was only ever dumped from main()'s panic handler, so the trace
// ring of a session that was merely slow or stuck -- the case a runtime trace
// is for -- could not be looked at without crashing the process. The
// features flag's own comment promised this command.
func (m Model) handleCmdFlightrec(input string, parts []string) (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: flightrecReport(m.workspace),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// flightrecReport performs the dump and says what happened.
func flightrecReport(workspace string) string {
	if !observability.FlightRecorderEnabled() {
		return "The flight recorder is not running. It starts with the process when " +
			"`features.flight_recorder` is true in .nerd/config.json or CODENERD_FLIGHT_RECORDER=1 is set; " +
			"campaign invocations never start it."
	}
	path, err := observability.DumpFlightRecord(workspace)
	if err != nil {
		return fmt.Sprintf("Flight recorder dump failed: %v", err)
	}
	return fmt.Sprintf("Flight trace written to `%s`.\nOpen it with `go tool trace %s`.", path, path)
}
