package features

import "testing"

func TestDefaultFeaturesConfig(t *testing.T) {
	c := DefaultFeaturesConfig()
	// Cheap/safe paths default on; experimental/expensive ones default off.
	// FlightRecorder is OFF by default: it drives the execution tracer and
	// can OOM the process under heavy load, so it is opt-in only.
	on := map[string]*bool{"SystemShards": c.SystemShards, "TaxonomyFast": c.TaxonomyFast}
	for name, p := range on {
		if p == nil || !*p {
			t.Errorf("%s should default to true", name)
		}
	}
	off := map[string]*bool{"FlightRecorder": c.FlightRecorder, "DiffEval": c.DiffEval, "Provenance": c.Provenance, "PerShardFacts": c.PerShardFacts, "DarkMode": c.DarkMode, "SkipOnboarding": c.SkipOnboarding}
	for name, p := range off {
		if p == nil || *p {
			t.Errorf("%s should default to false", name)
		}
	}
}

func TestParseInt64(t *testing.T) {
	ok := map[string]int64{"1": 1, "42": 42, "999": 999}
	for in, want := range ok {
		got, err := parseInt64(in)
		if err != nil || got != want {
			t.Errorf("parseInt64(%q)=(%d,%v), want (%d,nil)", in, got, err, want)
		}
	}
	// Non-digit characters, empty string, and non-positive values are rejected.
	for _, in := range []string{"", "0", "12a", "-5", " 7", "1.5"} {
		if _, err := parseInt64(in); err == nil {
			t.Errorf("parseInt64(%q) should return an error", in)
		}
	}
}

func TestFeaturesErrError(t *testing.T) {
	if errBadInt.Error() != "features: invalid integer override" {
		t.Errorf("unexpected error string: %q", errBadInt.Error())
	}
}
