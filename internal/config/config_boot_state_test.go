package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/features"
	"codenerd/internal/mcp"

	"github.com/stretchr/testify/require"
)

func TestLoadUserConfig_BootStateAbsentInstallsCoherentDefaults(t *testing.T) {
	configBootStateFixture(t)
	explicit := configBootStateNondefaultConfig()
	explicitPath, original := configBootStateWrite(t, explicit)
	absentWorkspace := t.TempDir()
	absentPath := filepath.Join(absentWorkspace, ".nerd", "config.json")

	for transition := 0; transition < 3; transition++ {
		accepted, err := LoadUserConfig(explicitPath)
		require.NoError(t, err)
		require.Equal(t, explicit, accepted)
		configBootStateAssertPolicy(t, accepted)
		configBootStateAssertFeatures(t, accepted)
		require.True(t, features.IsFlightRecorderEnabled())
		require.False(t, features.IsSystemShardsEnabled())
		require.Equal(t, 47*time.Second, GetLLMTimeouts().PerCallTimeout)
		require.Zero(t, GetLLMTimeouts().MaxRetries)

		defaults, err := LoadUserConfig(absentPath)
		require.NoError(t, err)
		require.Equal(t, &UserConfig{}, defaults)
		configBootStateAssertPolicy(t, defaults)
		configBootStateAssertFeatures(t, defaults)
		require.Equal(t, DefaultLLMTimeouts(), GetLLMTimeouts())
		_, err = os.Stat(absentPath)
		require.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Dir(absentPath))
		require.True(t, os.IsNotExist(err), "loading defaults must not create the workspace config directory")
		unchanged, err := os.ReadFile(explicitPath)
		require.NoError(t, err)
		require.Equal(t, original, unchanged)
	}
}

func TestLoadUserConfig_BootStateRejectedFilesPreserveAcceptedPolicy(t *testing.T) {
	configBootStateFixture(t)
	explicitPath, original := configBootStateWrite(t, configBootStateNondefaultConfig())
	accepted, err := LoadUserConfig(explicitPath)
	require.NoError(t, err)
	configBootStateAssertPolicy(t, accepted)
	configBootStateAssertFeatures(t, accepted)
	before := configBootStateSnapshot()
	cases := []struct {
		name string
		data string
		want string
	}{
		{"malformed", `{"features":{"flight_recorder":false},"llm_timeouts":`, ""},
		{"unknown", `{"features":{"flight_recorder":false,"unknown_flag":true}}`, "unknown field"},
		{"trailing", `{"features":{"flight_recorder":false}} {}`, ""},
		{"provider", `{"features":{"flight_recorder":false},"provider":"invalid-provider"}`, "provider"},
		{"timeout_profile", `{"features":{"flight_recorder":false},"llm_timeouts":{"profile":"invalid-profile"}}`, "llm_timeouts.profile"},
		{"timeout_duration", `{"features":{"flight_recorder":false},"llm_timeouts":{"per_call_timeout":"soon"}}`, "llm_timeouts.per_call_timeout"},
		{"timeout_nonpositive", `{"features":{"flight_recorder":false},"llm_timeouts":{"http_client_timeout":"0s"}}`, "llm_timeouts.http_client_timeout"},
		{"timeout_retries", `{"features":{"flight_recorder":false},"llm_timeouts":{"max_retries":-1}}`, "llm_timeouts.max_retries"},
		{"embedding", `{"features":{"flight_recorder":false},"embedding":{"request_timeout":"soon"}}`, "embedding.request_timeout"},
		{"embedding_pull", `{"features":{"flight_recorder":false},"embedding":{"pull_timeout":"0s"}}`, "embedding.pull_timeout"},
		{"research", `{"features":{"flight_recorder":false},"research":{"web_fetch_timeout":"soon"}}`, "research.web_fetch_timeout"},
		{"integrations", `{"features":{"flight_recorder":false},"integrations":{"default_timeout":"0s"}}`, "integrations.default_timeout"},
		{"core_limits", `{"features":{"flight_recorder":false},"core_limits":{"max_concurrent_shards":0}}`, "max_concurrent_shards"},
		{"articulation", `{"features":{"flight_recorder":false},"articulation":{"session_context_share_percent":101}}`, "articulation"},
		{"removed_feature", `{"features":{"flight_recorder":false,"diff_eval":true}}`, "features.diff_eval"},
		{"removed_timeout", `{"features":{"flight_recorder":false},"llm_timeouts":{"ooda_loop_timeout":"1m"}}`, "llm_timeouts.ooda_loop_timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.data), 0o600))
			got, err := LoadUserConfig(path)
			require.Error(t, err)
			require.Nil(t, got)
			if tc.want != "" {
				require.Contains(t, err.Error(), tc.want)
			}
			require.Equal(t, before, configBootStateSnapshot(), "rejected policy must publish nothing")
			unchanged, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, tc.data, string(unchanged))
		})
	}
	unchanged, err := os.ReadFile(explicitPath)
	require.NoError(t, err)
	require.Equal(t, original, unchanged)
}

func TestLoadUserConfig_BootStateEnvironmentPrecedenceSurvivesDefaultInstall(t *testing.T) {
	configBootStateFixture(t)
	path, _ := configBootStateWrite(t, configBootStateNondefaultConfig())
	t.Setenv("NERD_FLIGHTREC", "1")
	t.Setenv("CODENERD_FLIGHT_RECORDER", "0")
	t.Setenv("NERD_SKIP_ONBOARDING", "0")
	t.Setenv("CODENERD_SKIP_ONBOARDING", "1")
	t.Setenv("CODENERD_SYSTEM_SHARDS", "0")
	t.Setenv("NERD_FAST_SCAN_WORKERS", "11")
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "13")
	t.Setenv("NERD_FAST_AST_MAX_BYTES", "2048")
	t.Setenv("CODENERD_FAST_AST_MAX_BYTES", "4096")
	accepted, err := LoadUserConfig(path)
	require.NoError(t, err)
	configBootStateAssertPolicy(t, accepted)
	require.True(t, *accepted.Features.FlightRecorder)

	absentPath := filepath.Join(t.TempDir(), "config.json")
	for _, configPath := range []string{path, absentPath, path, absentPath} {
		cfg, err := LoadUserConfig(configPath)
		require.NoError(t, err)
		configBootStateAssertPolicy(t, cfg)
		require.False(t, features.IsFlightRecorderEnabled())
		require.True(t, features.IsOnboardingSkipped())
		require.False(t, features.IsSystemShardsEnabled())
		require.Equal(t, 13, features.FastScanWorkers())
		require.Equal(t, int64(4096), features.FastASTMaxBytes())
		for _, flag := range features.Resolved() {
			if flag.Name == "flight_recorder" || flag.Name == "skip_onboarding" || flag.Name == "system_shards" {
				require.Equal(t, features.SourceEnv, flag.Source)
			}
		}
	}

	t.Setenv("CODENERD_FLIGHT_RECORDER", "")
	t.Setenv("CODENERD_SKIP_ONBOARDING", "")
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "")
	t.Setenv("CODENERD_FAST_AST_MAX_BYTES", "")
	defaults, err := LoadUserConfig(absentPath)
	require.NoError(t, err)
	require.True(t, features.IsFlightRecorderEnabled())
	require.False(t, features.IsOnboardingSkipped())
	require.Equal(t, 11, features.FastScanWorkers())
	require.Equal(t, int64(2048), features.FastASTMaxBytes())
	for _, flag := range features.Resolved() {
		if flag.Name == "flight_recorder" || flag.Name == "skip_onboarding" {
			require.Equal(t, features.SourceLegacyEnv, flag.Source)
		}
	}

	t.Setenv("NERD_FLIGHTREC", "")
	t.Setenv("NERD_SKIP_ONBOARDING", "")
	t.Setenv("CODENERD_SYSTEM_SHARDS", "")
	t.Setenv("NERD_FAST_SCAN_WORKERS", "")
	t.Setenv("NERD_FAST_AST_MAX_BYTES", "")
	configBootStateAssertPolicy(t, defaults)
	configBootStateAssertFeatures(t, defaults)
	_, err = os.Stat(absentPath)
	require.True(t, os.IsNotExist(err))
}

type configBootState struct {
	flags          []features.Flag
	workers        int
	astBytes       int64
	llm            LLMTimeouts
	readBytes      int64
	searchBytes    int64
	research       ResearchPolicy
	embedRequest   time.Duration
	embedPull      time.Duration
	leafRequest    time.Duration
	leafPull       time.Duration
	image          time.Duration
	observation    ObservationLimits
	classification ClassificationHistory
	articulation   ArticulationConfig
	transport      time.Duration
}

func configBootStateSnapshot() configBootState {
	return configBootState{
		flags: features.Resolved(), workers: features.FastScanWorkers(), astBytes: features.FastASTMaxBytes(),
		llm: GetLLMTimeouts(), readBytes: ResolvedMaxReadFileBytes(), searchBytes: ResolvedMaxSearchFileBytes(),
		research: ResolvedResearchPolicy(), embedRequest: EmbeddingRequestTimeout(), embedPull: EmbeddingPullTimeout(),
		leafRequest: embedding.EmbedRequestTimeout(), leafPull: embedding.PullTimeout(), image: ImageRequestTimeout(),
		observation: ResolvedObservationLimits(), classification: ResolvedClassificationHistory(),
		articulation: ResolvedArticulationConfig(), transport: mcp.DefaultTransportTimeout(),
	}
}

func configBootStateFixture(t *testing.T) {
	t.Helper()
	for _, flag := range features.Resolved() {
		t.Setenv(flag.EnvVar, "")
		if flag.LegacyEnvVar != "" {
			t.Setenv(flag.LegacyEnvVar, "")
		}
	}
	for _, env := range []string{"CODENERD_FAST_SCAN_WORKERS", "NERD_FAST_SCAN_WORKERS", "CODENERD_FAST_AST_MAX_BYTES", "NERD_FAST_AST_MAX_BYTES"} {
		t.Setenv(env, "")
	}
	before := configBootStateSnapshot()
	featureValues := map[string]any{}
	for _, flag := range before.flags {
		if flag.Source == features.SourceConfig {
			featureValues[flag.Name] = flag.Value
		}
	}
	if before.workers != 0 {
		featureValues["fast_scan_workers"] = before.workers
	}
	if before.astBytes != 0 {
		featureValues["fast_ast_max_bytes"] = before.astBytes
	}
	var priorFeatures *features.FeaturesConfig
	if len(featureValues) > 0 {
		data, err := json.Marshal(featureValues)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &priorFeatures))
	}
	t.Cleanup(func() {
		features.SetActive(priorFeatures)
		SetLLMTimeouts(before.llm)
		SetExecutionFileLimits(ExecutionConfig{MaxReadFileBytes: before.readBytes, MaxSearchFileBytes: before.searchBytes})
		SetResearchPolicy(before.research)
		SetEmbeddingRequestTimeout(before.embedRequest)
		SetEmbeddingPullTimeout(before.embedPull)
		embedding.SetEmbedRequestTimeout(before.leafRequest)
		embedding.SetPullTimeout(before.leafPull)
		SetImageRequestTimeout(before.image)
		SetObservationLimits(before.observation)
		SetClassificationHistory(before.classification)
		SetArticulationConfig(before.articulation)
		mcp.SetTransportTimeoutFallback(before.transport)
	})
}

func configBootStateAssertPolicy(t *testing.T, cfg *UserConfig) {
	t.Helper()
	timeouts, err := cfg.LLMTimeouts.Resolve()
	require.NoError(t, err)
	require.Equal(t, timeouts, GetLLMTimeouts())
	execution := cfg.GetExecution()
	require.Equal(t, execution.MaxReadFileBytes, ResolvedMaxReadFileBytes())
	require.Equal(t, execution.MaxSearchFileBytes, ResolvedMaxSearchFileBytes())
	research, err := cfg.GetResearchConfig().Resolve()
	require.NoError(t, err)
	require.Equal(t, research, ResolvedResearchPolicy())
	emb := cfg.GetEmbeddingConfig()
	request, err := emb.ResolvedRequestTimeout()
	require.NoError(t, err)
	pull, err := emb.ResolvedPullTimeout()
	require.NoError(t, err)
	require.Equal(t, request, EmbeddingRequestTimeout())
	require.Equal(t, request, embedding.EmbedRequestTimeout())
	require.Equal(t, pull, EmbeddingPullTimeout())
	require.Equal(t, pull, embedding.PullTimeout())
	require.Equal(t, time.Duration(cfg.GetImageLLMConfig().Timeout)*time.Second, ImageRequestTimeout())
	require.Equal(t, cfg.GetObservationConfig().Resolve(), ResolvedObservationLimits())
	require.Equal(t, cfg.GetClassificationConfig().Resolve(), ResolvedClassificationHistory())
	require.Equal(t, cfg.GetArticulationConfig(), ResolvedArticulationConfig())
	transport, err := cfg.GetIntegrations().ResolveDefaultTimeout()
	require.NoError(t, err)
	require.Equal(t, transport, mcp.DefaultTransportTimeout())
}

func configBootStateAssertFeatures(t *testing.T, cfg *UserConfig) {
	t.Helper()
	data, err := json.Marshal(cfg.Features)
	require.NoError(t, err)
	var explicit map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &explicit))
	readers := map[string]bool{
		"flight_recorder": features.IsFlightRecorderEnabled(), "provenance": features.IsProvenanceEnabled(),
		"system_shards": features.IsSystemShardsEnabled(), "per_shard_facts": features.IsPerShardFactsEnabled(),
		"dark_mode": features.IsDarkModeEnabled(), "skip_onboarding": features.IsOnboardingSkipped(),
		"taxonomy_fast": features.IsTaxonomyFastEnabled(), "prompt_evolution": features.IsPromptEvolutionEnabled(),
	}
	for _, flag := range features.Resolved() {
		want, source := flag.Default, features.SourceDefault
		if raw, ok := explicit[flag.Name]; ok {
			require.NoError(t, json.Unmarshal(raw, &want))
			source = features.SourceConfig
		}
		require.Equal(t, want, flag.Value, flag.Name)
		require.Equal(t, source, flag.Source, flag.Name)
		require.Contains(t, readers, flag.Name)
		require.Equal(t, want, readers[flag.Name], flag.Name)
	}
	workers, astBytes := 0, int64(0)
	if cfg.Features != nil {
		workers, astBytes = cfg.Features.FastScanWorkers, cfg.Features.FastASTMaxBytes
	}
	require.Equal(t, workers, features.FastScanWorkers())
	require.Equal(t, astBytes, features.FastASTMaxBytes())
}

func configBootStateWrite(t *testing.T, cfg *UserConfig) (string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".nerd", "config.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path, data
}

func configBootStateNondefaultConfig() *UserConfig {
	zero := 0
	return &UserConfig{
		Features: &features.FeaturesConfig{
			FlightRecorder: boolPtr(true), Provenance: boolPtr(true), SystemShards: boolPtr(false),
			PerShardFacts: boolPtr(true), DarkMode: boolPtr(true), SkipOnboarding: boolPtr(true),
			TaxonomyFast: boolPtr(true), PromptEvolution: boolPtr(true), FastScanWorkers: 7, FastASTMaxBytes: 123456,
		},
		LLMTimeouts: &LLMTimeoutsConfig{
			Profile: "fast", HTTPClientTimeout: "41s", SlotAcquisitionTimeout: "59s", PerCallTimeout: "47s",
			StreamingTimeout: "61s", RetryBackoffBase: "25ms", RetryBackoffMax: "250ms", RateLimitDelay: "10ms",
			ArticulationTimeout: "23s", FollowUpTimeout: "29s", MaxRetries: &zero,
		},
		Execution: &ExecutionConfig{MaxReadFileBytes: 8192, MaxSearchFileBytes: 12288},
		Research: &ResearchConfig{
			WebFetchTimeout: "7s", Context7Timeout: "8s", WebSearchTimeout: "9s", BrowserExtractTimeout: "6s",
			BrowserExtractMaxChars: 2000, BrowserExtractMaxCharsCap: 3000,
			BrowserReasonItems: 7, BrowserReasonCompactItems: 3, BrowserHeadless: boolPtr(false),
		},
		Embedding: &EmbeddingConfig{RequestTimeout: "11s", PullTimeout: "19s"},
		Image:     &ImageLLMConfig{Timeout: 17},
		Observation: &ObservationConfig{
			MaxRegionLines: 90, PadLines: 9, MaxOutline: 17, MaxRegionBytes: 1024,
			SubagentHydrateMaxLines: 40, SubagentHydrateDefaultLines: 15,
		},
		Classification: &ClassificationConfig{HistoryTurnWindow: &zero, HistoryCharBudget: 1234},
		Articulation:   &ArticulationConfig{SessionContextSharePercent: 33},
		Integrations:   &IntegrationsConfig{DefaultTimeout: "13s"},
	}
}
