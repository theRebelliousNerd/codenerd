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
