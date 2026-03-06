package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestCodexCLIClient_RunHealthProbe_Success(t *testing.T) {
	fakeDir := writeFakeCodex(t)
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
	fakeDir := writeFakeCodex(t)
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
	fakeDir := writeFakeCodex(t)
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
	fakeDir := writeFakeCodex(t)
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

func writeFakeCodex(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "codex.cmd")
		script := "@echo off\r\n" +
			"setlocal\r\n" +
			"set \"out=\"\r\n" +
			":parse\r\n" +
			"if \"%~1\"==\"\" goto done\r\n" +
			"if \"%~1\"==\"--output-last-message\" (\r\n" +
			"  set \"out=%~2\"\r\n" +
			"  shift\r\n" +
			"  shift\r\n" +
			"  goto parse\r\n" +
			")\r\n" +
			"shift\r\n" +
			"goto parse\r\n" +
			":done\r\n" +
			"if \"%CODEX_TEST_MODE%\"==\"rate_limit\" (\r\n" +
			"  >&2 echo 429 rate limit\r\n" +
			"  exit /b 1\r\n" +
			")\r\n" +
			"if \"%CODEX_TEST_MODE%\"==\"auth\" (\r\n" +
			"  >&2 echo please login\r\n" +
			"  exit /b 1\r\n" +
			")\r\n" +
			"if \"%CODEX_TEST_MODE%\"==\"schema_bad\" (\r\n" +
			"  > \"%out%\" echo not-json\r\n" +
			"  exit /b 0\r\n" +
			")\r\n" +
			"> \"%out%\" echo %CODEX_TEST_PAYLOAD%\r\n" +
			"exit /b 0\r\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake codex.cmd: %v", err)
		}
		return dir
	}

	path := filepath.Join(dir, "codex")
	script := "#!/usr/bin/env sh\n" +
		"out=\"\"\n" +
		"while [ \"$#\" -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"--output-last-message\" ]; then\n" +
		"    out=\"$2\"\n" +
		"    shift 2\n" +
		"    continue\n" +
		"  fi\n" +
		"  shift\n" +
		"done\n" +
		"case \"$CODEX_TEST_MODE\" in\n" +
		"  rate_limit)\n" +
		"    echo \"429 rate limit\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  auth)\n" +
		"    echo \"please login\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  schema_bad)\n" +
		"    printf '%s' 'not-json' > \"$out\"\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"printf '%s' \"$CODEX_TEST_PAYLOAD\" > \"$out\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}
	return dir
}
