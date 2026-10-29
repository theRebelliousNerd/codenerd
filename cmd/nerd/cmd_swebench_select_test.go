package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A dataset file holding one pretty-printed instance object is neither a JSON
// array nor JSONL; `nerd swebench` accepts it as a one-instance dataset
// through swebench.LoadInstance instead of rejecting it.
func TestSelectSwebenchInstance_AcceptsASinglePrettyPrintedInstance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.json")
	body := "{\n  \"instance_id\": \"org__repo-1\",\n  \"repo\": \"org/repo\",\n  \"base_commit\": \"abc123\",\n  \"FAIL_TO_PASS\": [\"t1\"]\n}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	inst, err := selectSwebenchInstance(path, "")
	if err != nil {
		t.Fatalf("selectSwebenchInstance: %v", err)
	}
	if inst.InstanceID != "org__repo-1" || inst.Repo != "org/repo" {
		t.Fatalf("instance = %+v", inst)
	}

	if _, err := selectSwebenchInstance(path, "other"); err == nil {
		t.Fatal("asking for an instance the file does not hold must fail")
	}
}
