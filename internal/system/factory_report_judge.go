package system

import (
	"codenerd/internal/config"
	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/turnjudge"
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

// stepTechnologyReaderFor is the planned-step technology reading
// (typesafe.step_technologies): Jev reads, per technology a step's file
// imports, whether the step's change writes it, before the step's experts
// are selected. Nil when the section or the use is off.
func stepTechnologyReaderFor(appCfg *config.UserConfig) experts.StepReader {
	if appCfg == nil || !appCfg.TypeSafe.Uses().StepTechnologies {
		return nil
	}
	jev, err := typesafe.FromConfig(appCfg.TypeSafe)
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Jev step technology reading off: %v", err)
		return nil
	}
	if jev == nil {
		return nil
	}
	return turnjudge.StepTechnologyReader{Asker: jev}
}
