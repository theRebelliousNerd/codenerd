package chat

import (
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/ux"
)

// The adaptive UX loop: the user's journey state (new -> learning ->
// productive -> power) chooses how much help /help shows, and it moves on the
// metrics UserMetrics.ShouldTransition reads. Every half of that loop existed
// -- the metrics, the transition rule, the help levels -- and nothing ever
// counted a session, a command, a success, an error, a clarification or a help
// request, so no user ever left the state their preferences file started in.
//
// One in-process writer owns the counts: m.preferencesMgr, loaded at boot.
// Counts accumulate in memory and are saved when the session opens and when it
// closes, so a turn never waits on a file write.

// uxPrefs is the session's preferences manager, or a freshly loaded one when
// the model has none (a legacy boot path). nil when neither can be had.
func (m Model) uxPrefs() *ux.PreferencesManager {
	if m.preferencesMgr != nil {
		return m.preferencesMgr
	}
	pm := ux.NewPreferencesManager(m.workspace)
	if err := pm.Load(); err != nil {
		logging.Get(logging.CategorySession).Warn("preferences not loaded: %v", err)
		return nil
	}
	return pm
}

// recordUXMetric counts one event of a UserMetrics kind.
func (m Model) recordUXMetric(metric string) {
	if m.preferencesMgr == nil {
		return
	}
	if err := m.preferencesMgr.IncrementMetric(metric); err != nil {
		logging.Get(logging.CategorySession).Warn("ux metric %s not counted: %v", metric, err)
	}
}

// openSessionRecord marks the start of a chat session, after the preferences
// migration ran: it reloads the preferences the migration may have rewritten
// (the boot-time copy predates it, and saving it would undo the migration),
// counts the session, and opens the session in the audit trail.
func (m Model) openSessionRecord(migration *ux.MigrationResult, migErr error) Model {
	log := logging.Get(logging.CategorySession)
	switch {
	case migErr != nil:
		log.Warn("preferences migration failed: %v", migErr)
	case migration != nil && migration.WasMigrated:
		log.Info("preferences migrated %q -> %q; kept %v; defaults %v",
			migration.FromVersion, migration.ToVersion, migration.PreservedData, migration.DefaultsApplied)
	}

	if m.preferencesMgr != nil {
		if err := m.preferencesMgr.Load(); err != nil {
			log.Warn("preferences reload after migration failed; session metrics will not be recorded: %v", err)
			m.preferencesMgr = nil
		} else if err := m.preferencesMgr.RecordSessionStart(); err != nil {
			log.Warn("session start not recorded in preferences: %v", err)
		}
	}
	logging.AuditWithSession(m.sessionID).SessionStart(m.sessionID)
	m.sessionOpenedAt = time.Now()
	return m
}

// closeSessionRecord persists the session's counts, applies any journey
// transition they earned, and closes the session in the audit trail. A session
// that never opened (a boot that failed, a test model) records nothing.
func (m *Model) closeSessionRecord() {
	if m.sessionOpenedAt.IsZero() {
		return
	}
	log := logging.Get(logging.CategorySession)
	if m.preferencesMgr != nil {
		if err := m.preferencesMgr.Save(); err != nil {
			log.Warn("session metrics not saved: %v", err)
		} else if state, moved, err := m.preferencesMgr.CheckJourneyTransition(); err != nil {
			log.Warn("journey transition check failed: %v", err)
		} else if moved {
			log.Info("user journey moved to %s", state)
		}
	}
	logging.AuditWithSession(m.sessionID).SessionEnd(m.sessionID, m.turnCount, time.Since(m.sessionOpenedAt).Milliseconds())
}
