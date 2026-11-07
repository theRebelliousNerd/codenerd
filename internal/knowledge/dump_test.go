package knowledge

import (
	"os"
	"testing"
)

func TestDumpAcquire(t *testing.T) {
	for _, f := range []string{"internal/knowledge/acquire.go", "internal/knowledge/measure.go", "internal/knowledge/sources.go", "internal/knowledge/usage.go", "internal/knowledge/wiring.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Logf("read %s err %v", f, err)
			continue
		}
		t.Logf("FILE %s len %d head: %.4000s", f, len(b), string(b))
	}
}
