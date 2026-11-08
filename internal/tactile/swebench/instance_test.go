package swebench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadInstancesJSONAndJSONL(t *testing.T) {
	instances := []*Instance{
		{InstanceID: "one", Repo: "org/repo"},
		{InstanceID: "two", Repo: "org/repo2"},
	}
	data, err := json.Marshal(instances)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "instances.json")
	if err := writeFile(path, data); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loaded, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("failed to load instances: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(loaded))
	}

	jsonl := []byte(`{"instance_id":"a","repo":"org/a"}` + "\n" + `{"instance_id":"b","repo":"org/b"}` + "\n")
	path = filepath.Join(t.TempDir(), "instances.jsonl")
	if err := writeFile(path, jsonl); err != nil {
		t.Fatalf("failed to write jsonl: %v", err)
	}
	loaded, err = LoadInstances(path)
	if err != nil {
		t.Fatalf("failed to load jsonl: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 jsonl instances, got %d", len(loaded))
	}
}

func TestInstanceHelpers(t *testing.T) {
	instance := &Instance{
		InstanceID: "id",
		Repo:       "owner/project",
		FailToPass: []string{"a"},
		PassToPass: []string{"b", "c"},
	}

	if instance.RepoOwner() != "owner" {
		t.Fatalf("unexpected repo owner")
	}
	if instance.RepoName() != "project" {
		t.Fatalf("unexpected repo name")
	}
	if instance.GitURL() != "https://github.com/owner/project.git" {
		t.Fatalf("unexpected git url")
	}
	if count := instance.TestCount(); count != 3 {
		t.Fatalf("unexpected test count: %d", count)
	}
	if all := instance.AllTests(); len(all) != 3 {
		t.Fatalf("unexpected all tests length")
	}
	if !strings.Contains(instance.String(), "Instance{ID: id") {
		t.Fatalf("unexpected string output")
	}
}

func TestEvaluationResultRates(t *testing.T) {
	result := &EvaluationResult{
		InstanceID: "id",
		FailToPassResults: map[string]TestResult{
			"a": {Passed: true},
			"b": {Passed: false},
		},
		PassToPassResults: map[string]TestResult{
			"c": {Passed: true},
		},
		Duration: 3 * time.Second,
	}

	if rate := result.FailToPassRate(); rate != 50 {
		t.Fatalf("unexpected fail-to-pass rate: %v", rate)
	}
	if rate := result.PassToPassRate(); rate != 100 {
		t.Fatalf("unexpected pass-to-pass rate: %v", rate)
	}

	result.Resolved = false
	if !strings.Contains(result.Summary(), "[FAILED]") {
		t.Fatalf("expected failed summary")
	}
}

func TestPythonVersionHintsAndImage(t *testing.T) {
	instance := &Instance{Repo: "django/django"}
	hints := instance.PythonVersionHints()
	if len(hints) == 0 {
		t.Fatalf("expected python version hints")
	}
	if instance.PreferredPythonVersion() != "3.10" {
		t.Fatalf("unexpected preferred version: %s", instance.PreferredPythonVersion())
	}
	if instance.DockerImage() != "python:3.10-slim" {
		t.Fatalf("unexpected docker image: %s", instance.DockerImage())
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func TestInstance_NilReceiver_WhenNil_ShouldReturnZeroValues(t *testing.T) {
	var instance *Instance
	if got := instance.RepoOwner(); got != "" {
		t.Errorf("RepoOwner = %q, want empty", got)
	}
	if got := instance.RepoName(); got != "" {
		t.Errorf("RepoName = %q, want empty", got)
	}
	if got := instance.GitURL(); got != "" {
		t.Errorf("GitURL = %q, want empty", got)
	}
	if got := instance.AllTests(); got != nil {
		t.Errorf("AllTests = %v, want nil", got)
	}
	if got := instance.TestCount(); got != 0 {
		t.Errorf("TestCount = %d, want 0", got)
	}
	if got := instance.String(); got != "Instance{<nil>}" {
		t.Errorf("String = %q, want nil marker", got)
	}
	if hints := instance.PythonVersionHints(); len(hints) != 2 || hints[0] != "3.10" {
		t.Errorf("PythonVersionHints = %v, want default", hints)
	}
	if got := instance.PreferredPythonVersion(); got != "3.10" {
		t.Errorf("PreferredPythonVersion = %q, want 3.10", got)
	}
	if got := instance.DockerImage(); got != "python:3.10-slim" {
		t.Errorf("DockerImage = %q, want default", got)
	}
}

func TestEvaluationResult_NilReceiver_WhenNil_ShouldReturnSafeDefaults(t *testing.T) {
	var result *EvaluationResult
	if result.IsResolved() {
		t.Error("IsResolved = true, want false for nil")
	}
	if got := result.FailToPassRate(); got != 0.0 {
		t.Errorf("FailToPassRate = %v, want 0", got)
	}
	if got := result.PassToPassRate(); got != 100.0 {
		t.Errorf("PassToPassRate = %v, want 100", got)
	}
	if got := result.Summary(); !strings.Contains(got, "<nil>") {
		t.Errorf("Summary = %q, want nil marker", got)
	}
}

func TestInstance_GitURL_WhenEmptyRepo_ShouldReturnEmpty_InstanceCheck(t *testing.T) {
	instance := &Instance{Repo: ""}
	if got := instance.GitURL(); got != "" {
		t.Errorf("GitURL = %q, want empty for empty repo", got)
	}
}

func TestLoadInstances_WhenSecondLineInvalid_ShouldErrorWithLineNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "badline.jsonl")
	data := []byte("{\"instance_id\":\"a\",\"repo\":\"a/b\"}\nnot json\n")
	if err := writeFile(path, data); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := LoadInstances(path); err == nil {
		t.Fatal("expected error for invalid second line")
	} else if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected line 2 in error, got: %v", err)
	}
}

func TestLoadInstances_WhenEmptyFile_ShouldReturnEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := writeFile(path, []byte("")); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	loaded, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 instances, got %d", len(loaded))
	}
}

func TestLoadInstances_WhenMissingFile_ShouldError_InstanceCheck(t *testing.T) {
	if _, err := LoadInstances(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatal("expected error for missing file")
	} else if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("expected read error, got: %v", err)
	}
}

func TestLoadInstances_WhenBlankLines_ShouldSkipEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.jsonl")
	data := []byte("\n{\"instance_id\":\"a\",\"repo\":\"a/b\"}\n\n{\"instance_id\":\"b\",\"repo\":\"c/d\"}\n\n")
	if err := writeFile(path, data); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	loaded, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(loaded))
	}
	if loaded[0].InstanceID != "a" || loaded[1].InstanceID != "b" {
		t.Errorf("unexpected instances: %v %v", loaded[0].InstanceID, loaded[1].InstanceID)
	}
}

func TestLoadInstances_WhenWhitespaceOnly_ShouldReturnEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.jsonl")
	if err := writeFile(path, []byte("  \n\t\n")); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	loaded, err := LoadInstances(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 instances, got %d", len(loaded))
	}
}

func TestLoadInstance_WhenInvalidJSON_ShouldError_InstanceCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := writeFile(path, []byte("not json")); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := LoadInstance(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	} else if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestLoadInstance_WhenValidJSON_ShouldParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.json")
	if err := writeFile(path, []byte("{\"instance_id\":\"x\",\"repo\":\"o/r\"}")); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	inst, err := LoadInstance(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstanceID != "x" || inst.Repo != "o/r" {
		t.Errorf("unexpected instance: %+v", inst)
	}
	if got := inst.GitURL(); got != "https://github.com/o/r.git" {
		t.Errorf("GitURL = %q, want clone URL", got)
	}
}

func TestEvaluationResult_CountPassed_WhenNilOrEmpty_ShouldReturnZero(t *testing.T) {
	var nilResult *EvaluationResult
	if got := nilResult.countPassed(nil); got != 0 {
		t.Errorf("nil countPassed = %d, want 0", got)
	}
	r := &EvaluationResult{FailToPassResults: map[string]TestResult{}}
	if got := r.countPassed(r.FailToPassResults); got != 0 {
		t.Errorf("empty countPassed = %d, want 0", got)
	}
	r.FailToPassResults["a"] = TestResult{Passed: true}
	r.FailToPassResults["b"] = TestResult{Passed: false}
	if got := r.countPassed(r.FailToPassResults); got != 1 {
		t.Errorf("mixed countPassed = %d, want 1", got)
	}
}
