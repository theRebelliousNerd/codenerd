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

func TestLoadInstance_WhenValidJSON_ShouldParseBaseCommit(t *testing.T) {
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

func TestInstance_RepoOwner_WhenWithSlash_ShouldReturnOwner(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	if got := inst.RepoOwner(); got != "django" {
		t.Errorf("RepoOwner = %q, want %q", got, "django")
	}
}

func TestInstance_RepoName_WhenWithSlash_ShouldReturnName(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	if got := inst.RepoName(); got != "django" {
		t.Errorf("RepoName = %q, want %q", got, "django")
	}
}

func TestInstance_GitURL_ShouldContainRepo(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	got := inst.GitURL()
	if got != "https://github.com/django/django.git" {
		t.Errorf("GitURL = %q, want %q", got, "https://github.com/django/django.git")
	}
}

func TestInstance_AllTests_WhenBothSet_ShouldCombineInOrder(t *testing.T) {
	inst := &Instance{FailToPass: []string{"f1", "f2"}, PassToPass: []string{"p1"}}
	got := inst.AllTests()
	if len(got) != 3 || got[0] != "f1" || got[1] != "f2" || got[2] != "p1" {
		t.Errorf("AllTests = %v, want [f1 f2 p1]", got)
	}
	if n := inst.TestCount(); n != 3 {
		t.Errorf("TestCount = %d, want 3", n)
	}
}

func TestEvaluationResult_FailToPassRate_WhenEmpty_ShouldReturnZero(t *testing.T) {
	r := &EvaluationResult{}
	if got := r.FailToPassRate(); got != 0.0 {
		t.Errorf("FailToPassRate = %v, want 0", got)
	}
}

func TestEvaluationResult_PassToPassRate_WhenEmpty_ShouldReturnHundred(t *testing.T) {
	r := &EvaluationResult{}
	if got := r.PassToPassRate(); got != 100.0 {
		t.Errorf("PassToPassRate = %v, want 100", got)
	}
}

func TestEvaluationResult_PassToPassRate_WhenMixed_ShouldCompute(t *testing.T) {
	r := &EvaluationResult{
		PassToPassResults: map[string]TestResult{
			"p1": {Passed: true},
			"p2": {Passed: false},
		},
	}
	if got := r.PassToPassRate(); got != 50.0 {
		t.Errorf("PassToPassRate = %v, want 50", got)
	}
}

func TestInstance_PythonVersionHints_WhenKnownRepo_ShouldReturnMapped(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	hints := inst.PythonVersionHints()
	if len(hints) != 3 || hints[0] != "3.9" || hints[1] != "3.10" || hints[2] != "3.11" {
		t.Errorf("PythonVersionHints = %v, want [3.9 3.10 3.11]", hints)
	}
	unknown := &Instance{Repo: "unknown/repo"}
	defHints := unknown.PythonVersionHints()
	if len(defHints) != 2 || defHints[0] != "3.10" || defHints[1] != "3.11" {
		t.Errorf("PythonVersionHints(unknown) = %v, want [3.10 3.11]", defHints)
	}
}

func TestInstance_PreferredPythonVersion_WhenKnownRepo_ShouldReturnMiddle(t *testing.T) {
	inst := &Instance{Repo: "django/django"}
	if got := inst.PreferredPythonVersion(); got != "3.10" {
		t.Errorf("PreferredPythonVersion = %q, want %q", got, "3.10")
	}
}

func TestInstance_DockerImage_WhenUnknownRepo_ShouldUseDefaultVersion(t *testing.T) {
	inst := &Instance{Repo: "unknown/repo"}
	if got := inst.DockerImage(); got != "python:3.11-slim" {
		t.Errorf("DockerImage = %q, want %q", got, "python:3.11-slim")
	}
}

func TestLoadInstances_WhenMissingFile_ShouldError(t *testing.T) {
	if _, err := LoadInstances(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadInstances_WhenInvalidJSONL_ShouldError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(path, []byte("not json {{{"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadInstances(path); err == nil {
		t.Error("expected error for invalid JSONL")
	}
}

func TestLoadInstances_WhenJSONLWithBlankLines_ShouldSkipBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blanks.jsonl")
	data := "{\"instance_id\":\"a\",\"repo\":\"a/b\"}\n\n{\"instance_id\":\"b\",\"repo\":\"c/d\"}\n"
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

func TestEvaluationResult_Summary_WhenEmptyResults_ShouldContainCounts(t *testing.T) {
	r := &EvaluationResult{InstanceID: "x__y-1"}
	s := r.Summary()
	if !strings.Contains(s, "x__y-1") {
		t.Errorf("expected instance ID in summary, got: %s", s)
	}
	if !strings.Contains(s, "F2P=") || !strings.Contains(s, "P2P=") {
		t.Errorf("expected F2P/P2P rates in summary, got: %s", s)
	}
}

func TestInstance_DockerImage_ExactFormat_ShouldPinSlimDelegation(t *testing.T) {
	var nilInst *Instance
	if got := nilInst.DockerImage(); got != "python:3.10-slim" {
		t.Fatalf("DockerImage(nil) = %q, want %q", got, "python:3.10-slim")
	}
	repos := []string{"django/django", "psf/requests", "pallets/flask", "pytest-dev/pytest", "scikit-learn/scikit-learn", "unknown/unknown-repo-xyz"}
	for _, repo := range repos {
		inst := &Instance{Repo: repo}
		want := "python:" + inst.PreferredPythonVersion() + "-slim"
		if got := inst.DockerImage(); got != want {
			t.Fatalf("DockerImage() for repo %q = %q, want %q", repo, got, want)
		}
		if got := inst.DockerImage(); len(got) < 5 || got[len(got)-5:] != "-slim" {
			t.Fatalf("DockerImage() for repo %q = %q, want -slim suffix", repo, got)
		}
	}
}

func TestInstance_RepoOwner_WhenNil_ShouldReturnEmpty(t *testing.T) {
	var inst *Instance
	if got := inst.RepoOwner(); got != "" {
		t.Errorf("RepoOwner(nil) = %q, want %q", got, "")
	}
}

func TestInstance_RepoName_WhenNil_ShouldReturnEmpty(t *testing.T) {
	var inst *Instance
	if got := inst.RepoName(); got != "" {
		t.Errorf("RepoName(nil) = %q, want %q", got, "")
	}
}

func TestInstance_GitURL_WhenNil_ShouldReturnEmpty(t *testing.T) {
	var inst *Instance
	if got := inst.GitURL(); got != "" {
		t.Errorf("GitURL(nil) = %q, want %q", got, "")
	}
}

func TestInstance_GitURL_WhenEmptyRepo_ShouldReturnEmpty(t *testing.T) {
	inst := &Instance{}
	if got := inst.GitURL(); got != "" {
		t.Errorf("GitURL(empty repo) = %q, want %q", got, "")
	}
	emptyRepo := &Instance{Repo: ""}
	if got := emptyRepo.GitURL(); got != "" {
		t.Errorf("GitURL(empty repo) = %q, want %q", got, "")
	}
}

func TestInstance_AllTests_WhenNil_ShouldReturnNil(t *testing.T) {
	var inst *Instance
	got := inst.AllTests()
	if got != nil {
		t.Errorf("AllTests(nil) = %v, want nil", got)
	}
	if len(got) != 0 {
		t.Errorf("len(AllTests(nil)) = %d, want 0", len(got))
	}
}

func TestInstance_TestCount_WhenNil_ShouldReturnZero(t *testing.T) {
	var inst *Instance
	if got := inst.TestCount(); got != 0 {
		t.Errorf("TestCount(nil) = %d, want 0", got)
	}
}

func TestInstance_String_WhenNil_ShouldReturnNilMarker(t *testing.T) {
	var inst *Instance
	if got := inst.String(); got != "Instance{<nil>}" {
		t.Errorf("String(nil) = %q, want %q", got, "Instance{<nil>}")
	}
}

func TestEvaluationResult_IsResolved_WhenNil_ShouldReturnFalse(t *testing.T) {
	var r *EvaluationResult
	if r.IsResolved() {
		t.Error("IsResolved(nil) = true, want false")
	}
}

func TestEvaluationResult_FailToPassRate_WhenNil_ShouldReturnZero(t *testing.T) {
	var r *EvaluationResult
	if got := r.FailToPassRate(); got != 0.0 {
		t.Errorf("FailToPassRate(nil) = %v, want 0", got)
	}
}

func TestEvaluationResult_PassToPassRate_WhenNil_ShouldReturnHundred(t *testing.T) {
	var r *EvaluationResult
	if got := r.PassToPassRate(); got != 100.0 {
		t.Errorf("PassToPassRate(nil) = %v, want 100", got)
	}
}

func TestEvaluationResult_Summary_WhenNil_ShouldReturnNilMarker(t *testing.T) {
	var r *EvaluationResult
	got := r.Summary()
	if got != "[<nil>] <nil>: no result" {
		t.Errorf("Summary(nil) = %q, want %q", got, "[<nil>] <nil>: no result")
	}
	if !strings.Contains(got, "<nil>") {
		t.Errorf("expected '<nil>' in nil summary, got: %s", got)
	}
}

func TestInstance_PythonVersionHints_WhenNil_ShouldReturnDefault(t *testing.T) {
	var inst *Instance
	hints := inst.PythonVersionHints()
	if len(hints) != 2 || hints[0] != "3.10" || hints[1] != "3.11" {
		t.Errorf("PythonVersionHints(nil) = %v, want [3.10 3.11]", hints)
	}
}

func TestInstance_PreferredPythonVersion_WhenNil_ShouldReturnDefault(t *testing.T) {
	var inst *Instance
	if got := inst.PreferredPythonVersion(); got != "3.10" {
		t.Errorf("PreferredPythonVersion(nil) = %q, want %q", got, "3.10")
	}
}

func TestInstance_DockerImage_WhenNil_ShouldReturnDefaultSlim(t *testing.T) {
	var inst *Instance
	if got := inst.DockerImage(); got != "python:3.10-slim" {
		t.Errorf("DockerImage(nil) = %q, want %q", got, "python:3.10-slim")
	}
}

func TestInstance_PythonVersionHints_WhenFlaskRepo_ShouldPrefer39(t *testing.T) {
	inst := &Instance{Repo: "pallets/flask"}
	hints := inst.PythonVersionHints()
	if len(hints) != 3 || hints[0] != "3.8" || hints[1] != "3.9" || hints[2] != "3.10" {
		t.Fatalf("PythonVersionHints(flask) = %v, want [3.8 3.9 3.10]", hints)
	}
	if got := inst.PreferredPythonVersion(); got != "3.9" {
		t.Errorf("PreferredPythonVersion(flask) = %q, want %q", got, "3.9")
	}
	if got := inst.DockerImage(); got != "python:3.9-slim" {
		t.Errorf("DockerImage(flask) = %q, want %q", got, "python:3.9-slim")
	}
}
