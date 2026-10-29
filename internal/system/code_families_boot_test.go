package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/codefamily"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The boot check asserts a row for each declared member the workspace cannot
// resolve, and none for members it can.
func TestCheckCodeFamilyMembers_AssertsOnlyUnresolvedMembers(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":     "module fixture\n\ngo 1.22\n",
		"a/retry.go": "package a\n\nfunc Retry() {}\n",
		"b/retry.go": "package b\n\nfunc Retry() {}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	families := []codefamily.Family{{
		ID:                 "retry",
		Members:            []string{"fn:a.Retry", "fn:b.Retry", "fn:b.Renamed"},
		SharedRequirements: []string{"honor cancellation"},
	}}
	checkCodeFamilyMembers(context.Background(), root, k, families)
	rows, err := k.Query("code_family_member_unresolved")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || types.ExtractString(rows[0].Args[1]) != "fn:b.Renamed" {
		t.Fatalf("unresolved rows = %v; want only fn:b.Renamed", rows)
	}
}
