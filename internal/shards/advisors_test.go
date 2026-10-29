package shards

import "testing"

func TestGetStrategicAdvisorsFor(t *testing.T) {
	t.Parallel()

	advisors := GetStrategicAdvisorsFor("coder")
	if advisors == nil {
		t.Error("expected non-nil advisors slice")
	}
}
