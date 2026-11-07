package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

func lastAssistantContent(m Model) string {
	if len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1].Content
}

func TestHandleScanCompleteMsg_ChangedKeepsExistingTable(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	msg := scanCompleteMsg{
		fileCount:      10,
		directoryCount: 3,
		factCount:      42,
		duration:       time.Second,
	}
	got, _, _ := m.handleScanCompleteMsg(msg)
	content := lastAssistantContent(got)
	for _, want := range []string{
		"**Scan complete**",
		"| Files indexed | 10 |",
		"| Directories | 3 |",
		"| Facts generated | 42 |",
		"The kernel has been updated with fresh codebase facts.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("changed scan message missing %q\n got:\n%s", want, content)
		}
	}
	if strings.Contains(content, "unchanged") {
		t.Errorf("changed scan message must not mention unchanged, got:\n%s", content)
	}
}

func TestHandleScanCompleteMsg_UnchangedShowsRetainedKernelFacts(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	msg := scanCompleteMsg{
		fileCount:       7,
		directoryCount:  2,
		factCount:       0,
		duration:        500 * time.Millisecond,
		unchanged:       true,
		kernelFactCount: 123,
		hasKernelFacts:  true,
	}
	got, _, _ := m.handleScanCompleteMsg(msg)
	content := lastAssistantContent(got)
	for _, want := range []string{
		"unchanged",
		"| Files indexed | 7 |",
		"| Directories | 2 |",
		"| Kernel facts retained | 123 |",
		"no new facts generated",
		"kernel facts retained",
	} {
		if !strings.Contains(strings.ToLower(content), strings.ToLower(want)) && !strings.Contains(content, want) {
			t.Errorf("unchanged scan message missing %q\n got:\n%s", want, content)
		}
	}
	if strings.Contains(content, "Facts generated") {
		t.Errorf("unchanged message must not show a generated-facts row, got:\n%s", content)
	}
	if strings.Contains(content, "has been updated") {
		t.Errorf("unchanged message must not claim the kernel was updated, got:\n%s", content)
	}
}

func TestHandleScanCompleteMsg_UnchangedWithoutCountNeverFakesZero(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	// No kernel attached, no carried count: the count is unavailable.
	msg := scanCompleteMsg{
		fileCount:      5,
		directoryCount: 1,
		duration:       100 * time.Millisecond,
		unchanged:      true,
	}
	got, _, _ := m.handleScanCompleteMsg(msg)
	content := lastAssistantContent(got)
	if strings.Contains(content, "| Kernel facts retained | 0 |") || strings.Contains(content, "Facts generated | 0") {
		t.Errorf("unchanged message must never fake a zero count, got:\n%s", content)
	}
	if !strings.Contains(content, "unavailable") {
		t.Errorf("unchanged message without a kernel count should say unavailable, got:\n%s", content)
	}
}

func TestCountKernelFacts_NilKernelIsUnavailable(t *testing.T) {
	t.Parallel()
	if _, ok := countKernelFacts(nil); ok {
		t.Error("countKernelFacts(nil) = ok=true, want false so callers omit the count")
	}
}

func TestCountKernelFacts_RealKernelCountsFacts(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if _, ok := countKernelFacts(k); !ok {
		t.Fatal("empty real kernel should still report an available (possibly zero) count, not unavailable")
	}
}

// The retained count is the kernel's own fact count.
func TestCountKernelFacts_ReadsTheKernelsCount(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := k.FactCount()
	if err := k.Assert(core.Fact{Predicate: "file_topology", Args: []any{"a.go", "h", core.MangleAtom("/go"), int64(1), core.MangleAtom("/false")}}); err != nil {
		t.Fatal(err)
	}
	got, ok := countKernelFacts(k)
	if !ok || got != before+1 {
		t.Fatalf("countKernelFacts = %d, %v; want %d", got, ok, before+1)
	}
	if _, ok := countKernelFacts(nil); ok {
		t.Fatal("a nil kernel reported a count")
	}
}
