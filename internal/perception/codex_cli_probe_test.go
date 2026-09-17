package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestCodexCLIClient_RunHealthProbe_Success(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "success")
	t.Setenv("CODEX_TEST_PAYLOAD", `{"status":"ok","mode":"codex-exec-health","skill":"disabled","schema_valid":true}`)

	skillEnabled := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{SkillEnabled: &skillEnabled})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

func TestProbeCodexExec_MapsLoginFailures(t *testing.T) {
	fakeDir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(fakeDir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "auth")

	skillEnabled := false
	result, err := ProbeCodexExec(context.Background(), &config.CodexCLIConfig{SkillEnabled: &skillEnabled})
	if err == nil {
		t.Fatal("expected ProbeCodexExec to fail for login/auth errors")
	}
	if result.Classification != CodexExecProbeLoginRequired {
		t.Fatalf("Classification=%s, want %s", result.Classification, CodexExecProbeLoginRequired)
	}
}

func prependPath(first, existing string) string {
	if existing == "" {
		return first
	}
	return first + string(os.PathListSeparator) + existing
}
