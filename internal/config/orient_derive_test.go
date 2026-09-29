package config

import (
	"os"
	"path/filepath"
	"testing"
)

// orient.derive_request_bytes bounds one north-star derivation request. It is
// read through the orient section like every other orient key; a zero is
// absent, and a value outside 256..1048576 refuses the section.
func TestLoadOrientConfig_DeriveRequestBytes(t *testing.T) {
	def := DefaultOrientConfig().DeriveRequestBytes
	if def != 32<<10 {
		t.Fatalf("default derive_request_bytes = %d, want %d", def, 32<<10)
	}
	for _, tc := range []struct {
		name, body string
		value      int
		fail       bool
	}{
		{"absent", `{}`, def, false},
		{"other orient field", `{"orient":{"similar_top_k":5}}`, def, false},
		{"explicit", `{"orient":{"derive_request_bytes":4096}}`, 4096, false},
		{"zero is absent", `{"orient":{"derive_request_bytes":0}}`, def, false},
		{"negative", `{"orient":{"derive_request_bytes":-1}}`, 0, true},
		{"small", `{"orient":{"derive_request_bytes":255}}`, 0, true},
		{"large", `{"orient":{"derive_request_bytes":1048577}}`, 0, true},
		{"string", `{"orient":{"derive_request_bytes":"4096"}}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, ".nerd"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".nerd", "config.json"), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := LoadOrientConfig(dir)
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v, want failure=%v", err, tc.fail)
			}
			if err == nil && got.DeriveRequestBytes != tc.value {
				t.Fatalf("got %d want %d", got.DeriveRequestBytes, tc.value)
			}
		})
	}
}
