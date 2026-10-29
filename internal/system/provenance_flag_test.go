package system

import (
	"testing"

	"codenerd/internal/features"
)

// features.provenance is read by the kernel the session runs on. Until
// 2026-09-25 the flag parsed, resolved and was written true by `nerd init`,
// and nothing read it: recording came on only when /explain switched it on
// mid-session (GAP-FEAT-01, Docs/architecture/features/TODO.md).
func TestNewDomainCortex_HonorsTheProvenanceFlag(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })

	cases := []struct {
		env  string
		want bool
	}{
		{"1", true},
		{"0", false},
		{"", false}, // unset: the compile-time default is off
	}
	for _, tc := range cases {
		t.Run("CODENERD_PROVENANCE="+tc.env, func(t *testing.T) {
			t.Setenv("CODENERD_PROVENANCE", tc.env)
			if got := features.IsProvenanceEnabled(); got != tc.want {
				t.Fatalf("features.IsProvenanceEnabled() = %v, want %v", got, tc.want)
			}
			ck, err := NewDomainCortex(t.TempDir())
			if err != nil {
				t.Fatalf("NewDomainCortex: %v", err)
			}
			if got := ck.IsProvenanceEnabled(); got != tc.want {
				t.Errorf("the booted kernel records derivations = %v, want %v (features.provenance)", got, tc.want)
			}
		})
	}
}
