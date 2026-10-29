package system

import (
	"os"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
)

// configureSQLPragmas pushes the SQLite pragma settings that config and the
// logging switch own down into internal/sqlpragmas, which stays a leaf by not
// importing config.
//
// Host class: core_limits.sql_host_class, unless NERD_SQL_HOST_CLASS is set
// (the environment wins, as it does for every other override). An empty key
// clears a class an earlier Cortex in this process pushed, so a workspace that
// does not declare one gets the default rather than its neighbour's. Before
// this, SetHostClass had no caller and the class could only come from the
// environment.
//
// Failure metrics: recorded whenever logging.debug_mode is on. They are the
// observability half of pragma application -- the per-statement view is the
// driver's reject set -- and debug_mode is this system's observability switch.
// Recording is only ever switched on here; NERD_SQL_PRAGMA_METRICS=1 still
// turns it on without debug mode. /status reports what was recorded.
func configureSQLPragmas(hostClass string) {
	log := logging.Get(logging.CategoryStore)
	if _, envSet := os.LookupEnv(sqlpragmas.EnvHostClass); !envSet {
		switch hc, ok := sqlpragmas.ParseHostClass(hostClass); {
		case strings.TrimSpace(hostClass) == "":
			sqlpragmas.ClearHostClass()
		case ok:
			sqlpragmas.SetHostClass(hc)
		default:
			// LoadUserConfig refuses this value, so only a hand-built config
			// reaches here; say so rather than guess a class.
			log.Warn("core_limits.sql_host_class %q is not a host class (workstation, laptop, micro); SQLite stays at %s",
				hostClass, sqlpragmas.ActiveHostClass())
		}
	}
	if logging.IsDebugMode() {
		sqlpragmas.SetMetricsEnabled(true)
	}
	log.Info("SQLite host class: %s; pragma failure metrics: %v", sqlpragmas.ActiveHostClass(), sqlpragmas.MetricsEnabled())
}
