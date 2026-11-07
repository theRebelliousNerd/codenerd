package knowledge

import (
	"testing"
)

func TestCoverFillNilCtxDb(t *testing.T) {
	if _, _, err := Fill(nil, nil, Acquirer{}, nil); err == nil {
		t.Fatalf("want no kb error")
	}
}
