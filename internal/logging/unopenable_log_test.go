package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A category whose log file cannot be opened is reported once and then logs
// nowhere. Get used to leave the failure uncached, so every log call re-opened
// the file and printed the warning again: a campaign test that left logging
// bound to its deleted temp dir filled the package's test output with 201,908
// copies (45 MB), the repair round embedded that output in its task, and the
// request needed 12 million tokens (nerd fix A1, 2026-09-26). Not parallel: it
// binds the process's logging.
func TestGet_AnUnopenableLogFileIsReportedOnceNotOnEveryCall(t *testing.T) {
	dir := t.TempDir()
	ApplyConfig(Config{DebugMode: true, Level: "debug"})
	t.Cleanup(func() {
		CloseAll()
		ApplyConfig(Config{})
		ClearInjectedConfig()
	})
	if err := Initialize(dir); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	CloseAll()
	// The logs directory goes away under the process, as a test's temp dir
	// does when its cleanup runs.
	if err := os.RemoveAll(filepath.Join(dir, ".nerd", "logs")); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = w
	for i := 0; i < 3; i++ {
		Get(CategoryCampaign).Info("probe %d", i)
	}
	os.Stderr = stderr
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(out), "could not open log file"); n != 1 {
		t.Errorf("warned %d times for one unopenable log file, want once:\n%s", n, out)
	}
}
