package system

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/typesafe"
)

// reportJudgeFor is the session's final-report reader: Jev, when the typesafe
// section resolves and final_reports is on. Nil leaves the /finish round
// without a reading and the closure audit on the main model. A section that
// cannot resolve says why.
func reportJudgeFor(appCfg *config.UserConfig) *session.ReportJudge {
	if appCfg == nil || !appCfg.TypeSafe.Uses().FinalReports {
		return nil
	}
	jev, err := typesafe.FromConfig(appCfg.TypeSafe)
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Jev final-report reading off: %v", err)
		return nil
	}
	if jev == nil {
		return nil
	}
	return &session.ReportJudge{Asker: jev, Config: appCfg.TypeSafe}
}
