package config

import (
	"strings"
	"testing"
)

func TestCheck_ASecretPatternThatMatchesNothingRefusesTheFile(t *testing.T) {
	c := &UserConfig{Execution: &ExecutionConfig{SecretPaths: []string{".env", "[unclosed", "  "}}}
	var errs []string
	for _, p := range c.Check(nil) {
		if p.Severity == SeverityError && strings.HasPrefix(p.Path, "execution.secret_paths") {
			errs = append(errs, p.Path)
		}
	}
	if len(errs) != 2 {
		t.Fatalf("want errors for the malformed and the empty pattern, got %v", errs)
	}
}

func TestResolvedSecretPaths_AbsentMeansDefaultsAndEmptyMeansNone(t *testing.T) {
	var absent UserConfig
	absentExec := absent.GetExecution()
	if got := absentExec.ResolvedSecretPaths(); len(got) == 0 || got[0] != ".env" {
		t.Fatalf("no execution block must resolve to the defaults, got %v", got)
	}

	withBlock := UserConfig{Execution: &ExecutionConfig{AllowedBinaries: []string{"go"}}}
	blockExec := withBlock.GetExecution()
	if got := blockExec.ResolvedSecretPaths(); len(got) == 0 {
		t.Fatal("an execution block without secret_paths must resolve to the defaults")
	}

	none := UserConfig{Execution: &ExecutionConfig{SecretPaths: []string{}}}
	noneExec := none.GetExecution()
	if got := noneExec.ResolvedSecretPaths(); len(got) != 0 {
		t.Fatalf("an explicit [] is the user's decision and must resolve to none, got %v", got)
	}
}

func TestGetExecution_DefaultsComeFromOnePlace(t *testing.T) {
	want := DefaultExecutionConfig()
	got := (&UserConfig{Execution: &ExecutionConfig{}}).GetExecution()
	if strings.Join(got.AllowedEnvVars, ",") != strings.Join(want.AllowedEnvVars, ",") {
		t.Fatalf("allowed_env_vars fallback %v differs from DefaultExecutionConfig %v", got.AllowedEnvVars, want.AllowedEnvVars)
	}
	if strings.Join(got.AllowedBinaries, ",") != strings.Join(want.AllowedBinaries, ",") {
		t.Fatalf("allowed_binaries fallback %v differs from DefaultExecutionConfig %v", got.AllowedBinaries, want.AllowedBinaries)
	}
}
