package init

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/embedding"
)

func TestCreateMangleTemplatesPreservesUserContent(t *testing.T) {
	nerdDir := filepath.Join(t.TempDir(), ".nerd")
	mangleDir := filepath.Join(nerdDir, "mangle")
	if err := os.MkdirAll(mangleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	initializer := &Initializer{}
	if err := initializer.createMangleTemplates(nerdDir); err != nil {
		t.Fatalf("first createMangleTemplates() error = %v", err)
	}

	extensionsPath := filepath.Join(mangleDir, "extensions.mg")
	policyPath := filepath.Join(mangleDir, "policy_overrides.mg")
	const extensions = "# user extensions\nDecl user_owned(X).\n"
	const policy = "# user policy\npermitted(/custom).\n"
	if err := os.WriteFile(extensionsPath, []byte(extensions), 0o644); err != nil {
		t.Fatalf("WriteFile(extensions) error = %v", err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0o644); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}

	if err := initializer.createMangleTemplates(nerdDir); err != nil {
		t.Fatalf("second createMangleTemplates() error = %v", err)
	}
	assertFileContent(t, extensionsPath, extensions)
	assertFileContent(t, policyPath, policy)
}

func TestCreateDirectoryStructurePreservesUserGitignore(t *testing.T) {
	workspace := t.TempDir()
	initializer := &Initializer{config: InitConfig{Workspace: workspace}}
	nerdDir, err := initializer.createDirectoryStructure()
	if err != nil {
		t.Fatalf("first createDirectoryStructure() error = %v", err)
	}

	gitignorePath := filepath.Join(nerdDir, ".gitignore")
	const custom = "# user-owned ignore rules\ncustom.db\n"
	if err := os.WriteFile(gitignorePath, []byte(custom), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error = %v", err)
	}
	if _, err := initializer.createDirectoryStructure(); err != nil {
		t.Fatalf("second createDirectoryStructure() error = %v", err)
	}
	assertFileContent(t, gitignorePath, custom)
}

func TestInitializationContextAppliesConfiguredDeadline(t *testing.T) {
	ctx, cancel := initializationContext(context.Background(), 2*time.Second)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("initializationContext() did not install a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 2*time.Second {
		t.Fatalf("deadline remaining = %s, want (0s, 2s]", remaining)
	}
}

func TestInitializePreservesCancellationIdentity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	initializer := &Initializer{config: InitConfig{Timeout: time.Minute}}

	result, err := initializer.Initialize(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Initialize() error = %v, want errors.Is(context.Canceled)", err)
	}
	if result == nil || len(result.Failures) != 1 {
		t.Fatalf("Initialize() result = %+v, want one required cancellation failure", result)
	}
}

func TestEnsureEmbeddingEngineRejectsCorruptWorkspaceConfig(t *testing.T) {
	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	initializer := &Initializer{config: InitConfig{Workspace: workspace}}
	err := initializer.ensureEmbeddingEngine()
	if err == nil || !strings.Contains(err.Error(), "load workspace embedding config") {
		t.Fatalf("ensureEmbeddingEngine() error = %v, want corrupt workspace config failure", err)
	}
}

func TestWithJITPromptRecordsProviderFailure(t *testing.T) {
	initializer := &Initializer{
		config: InitConfig{LLMClient: &MockLLMClient{}},
		llmMetrics: InitLLMMetrics{
			Provider: "test-provider",
			Model:    "test-model",
		},
	}
	wantErr := errors.New("provider unavailable")

	_, gotErr := initializer.withJITPrompt(
		context.Background(),
		"analysis",
		"test metrics",
		nil,
		func(context.Context, string) (string, error) { return "", wantErr },
	)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("withJITPrompt() error = %v, want %v", gotErr, wantErr)
	}

	metrics := initializer.snapshotLLMMetrics()
	if metrics.Attempts != 1 || metrics.Succeeded != 0 || metrics.Failed != 1 {
		t.Fatalf("metrics = %+v, want one failed attempt", metrics)
	}
	if metrics.LastError != wantErr.Error() {
		t.Fatalf("LastError = %q, want %q", metrics.LastError, wantErr)
	}
	if got := formatLLMIdentity(metrics); got != "test-provider/test-model" {
		t.Fatalf("formatLLMIdentity() = %q", got)
	}
}

func TestRecordLLMCallIsConcurrentAndInternallyConsistent(t *testing.T) {
	initializer := &Initializer{}
	const calls = 100
	done := make(chan struct{}, calls)
	for n := 0; n < calls; n++ {
		go func(n int) {
			if n%2 == 0 {
				initializer.recordLLMCall(nil)
			} else {
				initializer.recordLLMCall(errors.New("failed"))
			}
			done <- struct{}{}
		}(n)
	}
	for n := 0; n < calls; n++ {
		<-done
	}

	metrics := initializer.snapshotLLMMetrics()
	if metrics.Attempts != calls || metrics.Succeeded != calls/2 || metrics.Failed != calls/2 {
		t.Fatalf("metrics = %+v, want %d attempts split evenly", metrics, calls)
	}
}

func TestInitializationSucceededRequiresNoRequiredFailures(t *testing.T) {
	if !initializationSucceeded(&InitResult{}) {
		t.Fatal("empty required-failure set should succeed")
	}
	if initializationSucceeded(&InitResult{Failures: []string{"database failed"}}) {
		t.Fatal("required failure must make initialization unsuccessful")
	}
	if initializationSucceeded(nil) {
		t.Fatal("nil result must not succeed")
	}
}

func TestRecordValidationOutcomeAffectsRequiredFailures(t *testing.T) {
	tests := []struct {
		name    string
		summary *ValidationSummary
		err     error
		failed  bool
	}{
		{name: "valid", summary: &ValidationSummary{TotalDBs: 3, ValidDBs: 3, OverallValid: true}},
		{name: "zero databases", summary: &ValidationSummary{OverallValid: true}, failed: true},
		{name: "invalid", summary: &ValidationSummary{TotalDBs: 3, InvalidDBs: 1}, failed: true},
		{name: "nil summary", failed: true},
		{name: "validator error", err: errors.New("read failed"), failed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &InitResult{}
			recordValidationOutcome(result, tt.summary, tt.err)
			if got := len(result.Failures) > 0; got != tt.failed {
				t.Fatalf("has failure = %t, want %t; failures=%v", got, tt.failed, result.Failures)
			}
		})
	}
}

func TestNewPhaseRunnerStartsAtOne(t *testing.T) {
	runner := newPhaseRunner(&Initializer{})
	if runner.phaseNum != 1 {
		t.Fatalf("phaseNum = %d, want 1", runner.phaseNum)
	}
}

func TestBoundedInitErrorNormalizesAndTruncates(t *testing.T) {
	got := boundedInitError(errors.New("line one\n\"api_key\": \"super-secret-value\" ghp_1234567890abcdef line two and more"), 64)
	if strings.Contains(got, "\n") {
		t.Fatalf("boundedInitError() retained newline: %q", got)
	}
	if strings.Contains(got, "super-secret-value") || strings.Contains(got, "ghp_1234567890abcdef") || !strings.Contains(got, "[redacted]") {
		t.Fatalf("boundedInitError() did not redact credential: %q", got)
	}
	if len([]rune(strings.TrimSuffix(got, "..."))) > 64 {
		t.Fatalf("boundedInitError() exceeded rune limit: %q", got)
	}
}

func TestInitEmbeddingModelUsesActiveProviderModel(t *testing.T) {
	if got := initEmbeddingModel(embedding.Config{Provider: "genai", GenAIModel: "gemini-embedding"}); got != "gemini-embedding" {
		t.Fatalf("genai model = %q", got)
	}
	if got := initEmbeddingModel(embedding.Config{Provider: "ollama", OllamaModel: "embeddinggemma"}); got != "embeddinggemma" {
		t.Fatalf("ollama model = %q", got)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if got := string(content); got != want {
		t.Fatalf("content of %q = %q, want %q", path, got, want)
	}
}
