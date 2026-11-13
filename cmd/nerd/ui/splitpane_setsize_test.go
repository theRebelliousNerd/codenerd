package ui

import "testing"

// A refit to the size the pane already has keeps its last render at hand: the
// chat refits on an idle tick, and an unmoved pane has nothing to redo. A real
// resize drops it.
func TestLogicPane_SetSizeKeepsTheLastRenderWhenNothingMoved(t *testing.T) {
	p := NewLogicPane(DefaultStyles(), 40, 20)
	p.SetSize(40, 20)
	p.renderCache.Render([]any{"trace", 40, 20}, func() string { return "tree" })
	if p.renderCache.lastKey == 0 {
		t.Fatal("the render was not remembered")
	}
	p.SetSize(40, 20)
	if p.renderCache.lastKey == 0 {
		t.Fatal("a refit to the same size dropped the pane's last render")
	}
	p.SetSize(60, 20)
	if p.renderCache.lastKey != 0 {
		t.Fatal("a real resize must drop the pane's last render")
	}
}
