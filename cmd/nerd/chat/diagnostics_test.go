package chat

import (
	"database/sql"
	"strings"
	"testing"

	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"

	_ "github.com/mattn/go-sqlite3"
)

// /status reports the in-memory state no log file shows while the session is
// running. Each of these readers existed with no caller: a pragma the driver
// rejected was counted and never shown, and logs bound to another workspace
// were discoverable only by not finding them. Not parallel: logging and the
// pragma metrics are process state.
func TestRenderDiagnostics_ShouldReportLoggingPragmasAndTheRecorder(t *testing.T) {
	logsHere := t.TempDir()
	if err := logging.Initialize(logsHere); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}

	wasOn := sqlpragmas.MetricsEnabled()
	sqlpragmas.SetMetricsEnabled(true)
	sqlpragmas.ResetPragmaMetrics()
	t.Cleanup(func() {
		sqlpragmas.ResetPragmaMetrics()
		sqlpragmas.SetMetricsEnabled(wasOn)
	})
	// A closed handle refuses every pragma, which is enough to be counted.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	report := renderDiagnostics(t.TempDir())
	for _, want := range []string{
		"### Diagnostics",
		"NOT this workspace", // logs are bound to logsHere, not the workspace asked about
		"SQLite host class:",
		"SQLite pragma failures:",
		"the driver rejected:",
		"Flight recorder: off",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("diagnostics lack %q:\n%s", want, report)
		}
	}
	if strings.Contains(renderDiagnostics(logsHere), "NOT this workspace") {
		t.Error("the bound workspace itself was reported as a mismatch")
	}
}

// /flightrec says why it cannot dump rather than failing silently when the
// recorder never started.
func TestFlightrecReport_WhenRecorderIsOff_ShouldSayHowToTurnItOn(t *testing.T) {
	got := flightrecReport(t.TempDir())
	if !strings.Contains(got, "not running") || !strings.Contains(got, "flight_recorder") {
		t.Fatalf("report = %q", got)
	}
}
