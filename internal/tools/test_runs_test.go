package tools

import (
	"context"
	"testing"
)

func TestRecordTestRun_RecordsOnlyIntoALog(t *testing.T) {
	// Without a log there is nowhere to record, and nothing panics.
	RecordTestRun(context.Background(), TestRun{Argv: []string{"go", "test"}})

	ctx, runs := WithTestRunLog(context.Background())
	if got := runs(); len(got) != 0 {
		t.Fatalf("a fresh log holds %v, want nothing", got)
	}
	RecordTestRun(ctx, TestRun{Argv: []string{"go", "test", "./..."}, ExitCode: 1})
	got := runs()
	if len(got) != 1 || got[0].ExitCode != 1 || got[0].Argv[1] != "test" {
		t.Fatalf("runs = %+v, want the one recorded run", got)
	}
	// What the log returns is a copy: a caller cannot rewrite the record.
	got[0].ExitCode = 0
	if runs()[0].ExitCode != 1 {
		t.Fatal("the log handed out its own storage")
	}
}

func TestIsTestCommand(t *testing.T) {
	for command, want := range map[string]bool{
		"go test ./...":        true,
		"GO TEST ./...":        true,
		"pytest":               true,
		"python -m pytest -q":  true,
		"cargo test":           true,
		"go test":              true,
		"go testify":           false,
		"go build ./...":       false,
		"ls -la":               false,
		"":                     false,
		"echo go test ./...":   false,
		"npm run test -- --ci": true,
	} {
		if got := IsTestCommand(command); got != want {
			t.Errorf("IsTestCommand(%q) = %v, want %v", command, got, want)
		}
	}
}
