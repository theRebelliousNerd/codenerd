package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The gate's whole job is telling compiled programs apart from everything
// else that is binary. Pin the magic table in both directions: every
// executable format trips it, and non-program bytes (including short and
// missing files) never do.

func TestExecutableKind(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"elf", []byte{0x7F, 'E', 'L', 'F', 0x02}, "ELF (Linux)"},
		{"pe", []byte{'M', 'Z', 0x90, 0x00}, "PE (Windows)"},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)"},
		{"macho32", []byte{0xCE, 0xFA, 0xED, 0xFE}, "Mach-O 32 (macOS)"},
		{"machouniv", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "Mach-O universal (macOS)"},
		{"png", []byte{0x89, 'P', 'N', 'G'}, ""},
		{"text", []byte("package main\n"), ""},
		{"empty", []byte{}, ""},
		{"short", []byte{'M'}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := executableKind(write(tc.name, tc.data))
			if tc.want == "" && ok {
				t.Errorf("executableKind(%s) = %q, want no match", tc.name, got)
			}
			if tc.want != "" && (!ok || got != tc.want) {
				t.Errorf("executableKind(%s) = (%q, %v), want (%q, true)", tc.name, got, ok, tc.want)
			}
		})
	}
}

func TestExecutableKind_MissingFile_IsNotTheGatesBusiness(t *testing.T) {
	if kind, ok := executableKind(filepath.Join(t.TempDir(), "nope")); ok {
		t.Errorf("missing file reported as %q", kind)
	}
}
