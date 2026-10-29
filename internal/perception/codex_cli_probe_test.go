package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

func TestCodexCLIClient_RunHealthProbe_Success(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "success")
	t.Setenv("CODEX_TEST_PAYLOAD", `{"status":"ok","mode":"codex-exec-health","skill":"disabled","schema_valid":true}`)

	skillEnabled := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{SkillEnabled: &skillEnabled})

	// The test's own context, not a wall clock: a fake CLI's startup on a
	// loaded Windows runner took 5.8s, and a 5s bound failed it as exec_failed.
	ctx := t.Context()

	result, err := client.RunHealthProbe(ctx)
	if err != nil {
		t.Fatalf("RunHealthProbe() error = %v", err)
	}
	if !result.SchemaValidated {
		t.Fatal("expected SchemaValidated=true")
	}
	if result.Failure != CodexCLIProbeFailureNone {
		t.Fatalf("Failure=%s, want no failure", result.Failure)
	}
}

func TestCodexCLIClient_RunHealthProbe_SkillMissingAfterSuccessfulExec(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "success")
	t.Setenv("CODEX_TEST_PAYLOAD", `{"status":"ok","mode":"codex-exec-health","skill":"disabled","schema_valid":true}`)

	client := NewCodexCLIClient(nil)
	client.skillEnabled = true
	client.skillAvailable = false
	client.skillName = config.DefaultCodexExecSkillName
	client.skillPath = filepath.Join(t.TempDir(), ".agents", "skills", client.skillName, "SKILL.md")

	ctx := t.Context()

	result, err := client.RunHealthProbe(ctx)
	if err == nil {
		t.Fatal("expected skill-missing probe to error")
	}
	if result.Failure != CodexCLIProbeFailureSkillMissing {
		t.Fatalf("Failure=%s, want %s", result.Failure, CodexCLIProbeFailureSkillMissing)
	}
	if !result.SchemaValidated {
		t.Fatal("expected SchemaValidated=true when exec/schema succeed before the skill check")
	}
}

func TestCodexCLIClient_RunHealthProbe_RateLimited(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "rate_limit")
	t.Setenv("CODEX_TEST_PAYLOAD", "")

	skillEnabled := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{SkillEnabled: &skillEnabled})

	ctx := t.Context()

	result, err := client.RunHealthProbe(ctx)
	if err == nil {
		t.Fatal("expected rate-limited probe to error")
	}
	if result.Failure != CodexCLIProbeFailureRateLimited {
		t.Fatalf("Failure=%s, want %s", result.Failure, CodexCLIProbeFailureRateLimited)
	}
}

func TestClassifyCodexCLIProbeError_AuthUnavailable(t *testing.T) {
	failure, detail := classifyCodexCLIProbeError(fmt.Errorf("please login to continue"))
	if failure != CodexCLIProbeFailureAuthUnavailable {
		t.Fatalf("failure=%s, want %s", failure, CodexCLIProbeFailureAuthUnavailable)
	}
	if detail == "" {
		t.Fatal("expected non-empty detail")
	}
}

// nerd auth codex / status read RunHealthProbe's Failure code directly, so the
// login-required case must surface as CodexCLIProbeFailureAuthUnavailable.
func TestRunHealthProbe_ClassifiesLoginFailures(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "auth")

	skillEnabled := false
	result, err := NewCodexCLIClient(&config.CodexCLIConfig{SkillEnabled: &skillEnabled}).RunHealthProbe(context.Background())
	if err == nil {
		t.Fatal("expected the health probe to fail for login/auth errors")
	}
	if result.Failure != CodexCLIProbeFailureAuthUnavailable {
		t.Fatalf("Failure=%s, want %s", result.Failure, CodexCLIProbeFailureAuthUnavailable)
	}
}

func prependPath(first, existing string) string {
	if existing == "" {
		return first
	}
	return first + string(os.PathListSeparator) + existing
}
