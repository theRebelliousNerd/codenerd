package system

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/retrieval"
	"codenerd/internal/turnjudge"
	"codenerd/internal/typesafe"
)

// taskRetrieverConfig is the session executor's per-task retrieval pass as
// the config sets it: retrieval's brief floor and, when the typesafe section
// turns retrieval relevance on, Jev judging Tier 2 with its thresholds beside
// the floor (policy/jev_evidence.mg lets a file Jev judged relevant into the
// brief). A typesafe section that cannot resolve leaves Jev off and says why;
// the pass runs on keywords as before.
func taskRetrieverConfig(appCfg *config.UserConfig, workspace string) retrieval.TaskRetrieverConfig {
	cfg := retrieval.TaskRetrieverConfig{
		WorkDir: workspace,
		Params:  appCfg.GetRetrievalConfig().Params(),
	}
	jev, err := typesafe.FromConfig(appCfg.TypeSafe)
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Jev retrieval relevance off for task passes: %v", err)
		return cfg
	}
	if jev == nil {
		return cfg
	}
	use := turnjudge.RelevanceFor(jev, appCfg.TypeSafe)
	if use.Judge == nil {
		return cfg
	}
	cfg.Relevance = use.Judge
	cfg.RelevanceDrop = use.Drop
	cfg.RelevancePoolFactor = use.PoolFactor
	cfg.RelevanceHeadBytes = use.HeadBytes
	cfg.Params = append(cfg.Params, appCfg.TypeSafe.Params()...)
	return cfg
}
