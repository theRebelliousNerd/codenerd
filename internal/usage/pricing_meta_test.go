package usage

import "testing"

// The Muse Spark contributor tier is priced at list ($0.10 in, $0.20 out,
// $0.002 cache-read per MTok); the standard tier has no rate on record and
// stays unpriced rather than borrowing the contributor's.
func TestLookupPrice_MuseSpark(t *testing.T) {
	tests := []struct {
		model  string
		want   Price
		priced bool
	}{
		{model: "muse-spark-1.3-contributor", want: Price{InputPerMTok: 0.10, OutputPerMTok: 0.20, CacheReadPerMTok: 0.002}, priced: true},
		{model: "meta/muse-spark-1.3-contributor", want: Price{InputPerMTok: 0.10, OutputPerMTok: 0.20, CacheReadPerMTok: 0.002}, priced: true},
		{model: "muse-spark-1.3"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got, ok := LookupPrice(tt.model)
			if ok != tt.priced || got != tt.want {
				t.Errorf("LookupPrice(%q) = (%+v, %v), want (%+v, %v)", tt.model, got, ok, tt.want, tt.priced)
			}
		})
	}
}
