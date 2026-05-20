package swebench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- EvaluationResult.Summary ---

func TestEvaluationResult_Summary_WhenResolved_ShouldShowResolved(t *testing.T) {
	r := &EvaluationResult{
		InstanceID: "django__django-11001",
		Resolved:   true,
		Duration:   5 * time.Second,
		FailToPassResults: map[string]TestResult{
			"test_1": {Passed: true},
		},
		PassToPassResults: map[string]TestResult{
			"test_2": {Passed: true},
		},
	}
	summary := r.Summary()
	if !strings.Contains(summary, "RESOLVED") {
		t.Errorf("expected 'RESOLVED' in summary, got: %s", summary)
	}
}

func TestEvaluationResult_Summary_WhenFailed_ShouldShowFailed(t *testing.T) {
	r := &EvaluationResult{
		InstanceID: "django__django-11001",
		Resolved:   false,
		Duration:   3 * time.Second,
		FailToPassResults: map[string]TestResult{
			"test_1": {Passed: false},
		},
	}
	summary := r.Summary()
	if !strings.Contains(summary, "FAILED") {
		t.Errorf("expected 'FAILED' in summary, got: %s", summary)
	}
}

func TestEvaluationResult_IsResolved_WhenFalse_ShouldReturnFalse(t *testing.T) {
	r := &EvaluationResult{Resolved: false}
	if r.IsResolved() {
		t.Error("expected false")
	}
}

func TestEvaluationResult_IsResolved_WhenTrue_ShouldReturnTrue(t *testing.T) {
	r := &EvaluationResult{Resolved: true}
	if !r.IsResolved() {
		t.Error("expected true")
	}
}

// --- Instance edge cases ---

func TestInstance_RepoOwner_WhenNoSlash_ShouldReturnFull(t *testing.T) {
	inst := &Instance{Repo: "myrepo"}
	if inst.RepoOwner() != "myrepo" {
		t.Errorf("RepoOwner = %q, want 'myrepo'", inst.RepoOwner())
	}
}

func TestInstance_RepoName_WhenNoSlash_ShouldReturnRepo(t *testing.T) {
	inst := &Instance{Repo: "myrepo"}
	if inst.RepoName() != "myrepo" {
		t.Errorf("RepoName = %q, want 'myrepo'", inst.RepoName())
	}
}

func TestInstance_AllTests_WhenBothEmpty_ShouldReturnEmpty(t *testing.T) {
	inst := &Instance{}
	if len(inst.AllTests()) != 0 {
		t.Error("expected empty AllTests")
	}
}

func TestInstance_TestCount_WhenBothEmpty_ShouldReturnZero(t *testing.T) {
	inst := &Instance{}
	if inst.TestCount() != 0 {
		t.Error("expected 0 TestCount")
	}
}

func TestInstance_String_ShouldContainFields(t *testing.T) {
	inst := &Instance{
		InstanceID: "test__id-001",
		Repo:       "test/repo",
		Version:    "1.0",
		FailToPass: []string{"t1"},
	}
	s := inst.String()
	if !strings.Contains(s, "test__id-001") {
		t.Errorf("expected InstanceID in string, got: %s", s)
	}
	if !strings.Contains(s, "test/repo") {
		t.Errorf("expected Repo in string, got: %s", s)
	}
}

// --- ParseJSONL / LoadInstance with temp files ---

func TestLoadInstance_WhenValidJSON_ShouldParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instance.json")
	data := `{"instance_id":"test__inst-001","repo":"test/repo","base_commit":"abc123"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	inst, err := LoadInstance(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstanceID != "test__inst-001" {
		t.Errorf("InstanceID = %q, want 'test__inst-001'", inst.InstanceID)
	}
}

func TestLoadInstance_WhenInvalidJSON_ShouldError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadInstance(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadInstance_WhenMissingFile_ShouldError(t *testing.T) {
	_, err := LoadInstance("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadInstances_WhenJSONL_ShouldParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instances.jsonl")
	data := `{"instance_id":"inst-001","repo":"a/b"}
{"instance_id":"inst-002","repo":"c/d"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	instances, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

func TestLoadInstances_WhenValidJSONArray_ShouldParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instances.json")
	data := `[{"instance_id":"inst-001","repo":"a/b"},{"instance_id":"inst-002","repo":"c/d"}]`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	instances, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

// --- Docker image ---

func TestInstance_DockerImage_ShouldContainPython(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	img := inst.DockerImage()
	if !strings.Contains(img, "python:") {
		t.Errorf("expected 'python:' in image, got: %s", img)
	}
	if !strings.Contains(img, "-slim") {
		t.Errorf("expected '-slim' in image, got: %s", img)
	}
}

// --- PreferredPythonVersion ---

func TestInstance_PreferredPythonVersion_WhenUnknownRepo_ShouldReturnDefault(t *testing.T) {
	inst := &Instance{Repo: "unknown/repo"}
	v := inst.PreferredPythonVersion()
	if v == "" {
		t.Error("expected non-empty version")
	}
}

// --- countPassed ---

func TestEvaluationResult_CountPassed_WhenMixed_ShouldCountCorrectly(t *testing.T) {
	r := &EvaluationResult{
		FailToPassResults: map[string]TestResult{
			"t1": {Passed: true},
			"t2": {Passed: false},
			"t3": {Passed: true},
		},
	}
	rate := r.FailToPassRate()
	if rate < 66.0 || rate > 67.0 {
		t.Errorf("FailToPassRate = %v, expected ~66.67", rate)
	}
}
