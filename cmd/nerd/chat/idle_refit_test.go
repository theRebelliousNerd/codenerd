package chat

import "testing"

// The two-second memory tick refits the layout only when the footer, the
// activity trail or the services section changed height: a refit with nothing
// moved was the commonest slow Update handler of a live session.
func TestMemUsageTick_RefitsOnlyWhenARowChangedHeight(t *testing.T) {
	m := NewTestModel()
	m = m.handleWindowSizeMsg(windowSizeMsg{Width: 100, Height: 30})
	fitted := m.viewport.Height
	m.idleHeights = m.idleRowHeights()

	// A height only a refit would correct: nothing changed, so it must stay.
	m.viewport.Height = 1
	updated, _ := m.Update(memUsageMsg{Alloc: 1 << 20, Sys: 2 << 20})
	m = updated.(Model)
	if m.viewport.Height != 1 {
		t.Fatalf("the tick refitted the layout with nothing moved (viewport height %d)", m.viewport.Height)
	}
	if m.memSysBytes != 2<<20 {
		t.Fatalf("the tick did not record the reading: %d", m.memSysBytes)
	}

	// A row that changed height since the last refit: the tick refits.
	m.idleHeights = [3]int{-1, -1, -1}
	updated, _ = m.Update(memUsageMsg{Alloc: 1 << 20, Sys: 2 << 20})
	m = updated.(Model)
	if m.viewport.Height != fitted {
		t.Fatalf("a changed row did not refit: viewport height %d, want %d", m.viewport.Height, fitted)
	}
	if m.idleHeights != m.idleRowHeights() {
		t.Fatal("the refit did not record the heights it fitted to")
	}
}
