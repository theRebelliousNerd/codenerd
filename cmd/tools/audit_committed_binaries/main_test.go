package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"codenerd/internal/executable"
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
		// Short-read pin: a 2-byte PE header fills fewer bytes than the
		// 4-byte window, so io.ReadFull reports ErrUnexpectedEOF. The
		// header prefix must still be examined (header[:n]) instead of
		// treating the short read as "not executable".
		{"pe-minimal", []byte{'M', 'Z'}, "PE (Windows)"},
		{"elf-truncated", []byte{0x7F, 'E', 'L'}, ""},
		{"png", []byte{0x89, 'P', 'N', 'G'}, ""},
		{"text", []byte("package main\n"), ""},
		{"empty", []byte{}, ""},
		{"short", []byte{'M'}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := executable.Kind(write(tc.name, tc.data))
			if tc.want == "" && ok {
				t.Errorf("executable.Kind(%s) = %q, want no match", tc.name, got)
			}
			if tc.want != "" && (!ok || got != tc.want) {
				t.Errorf("executable.Kind(%s) = (%q, %v), want (%q, true)", tc.name, got, ok, tc.want)
			}
		})
	}
}

func TestExecutableKind_MissingFile_IsNotTheGatesBusiness(t *testing.T) {
	if kind, ok := executable.Kind(filepath.Join(t.TempDir(), "nope")); ok {
		t.Errorf("missing file reported as %q", kind)
	}
}

func TestExecutableKind_TruncatedPrefixNeverMatches(t *testing.T) {
	dir := t.TempDir()
	truncated := [][]byte{
		{0x7F},
		{0x7F, 'E'},
		{0x7F, 'E', 'L'},
		{'M'},
		{0xCF},
		{0xCF, 0xFA},
		{0xCF, 0xFA, 0xED},
		{0xCE},
		{0xCE, 0xFA, 0xED},
		{0xCA},
		{0xCA, 0xFE},
		{0xCA, 0xFE, 0xBA},
	}
	for i, data := range truncated {
		p := filepath.Join(dir, "trunc")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if kind, ok := executable.Kind(p); ok {
			t.Errorf("truncated header %d (%q) reported as %q, want no match", i, data, kind)
		}
	}
	if kind, ok := executable.Kind(dir); ok {
		t.Errorf("executable.Kind(directory) = %q, want no match", kind)
	}
}

func TestExecutableKind_PinsHeaderPrefixSemantics(t *testing.T) {
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
		// Minimal exact headers must match. The 2-byte PE case is the
		// pin: a short read (io.ReadFull returns ErrUnexpectedEOF) must
		// still be examined instead of treated as "not executable".
		{"pe-minimal", []byte{'M', 'Z'}, "PE (Windows)"},
		{"elf-minimal", []byte{0x7F, 'E', 'L', 'F'}, "ELF (Linux)"},
		{"macho64-minimal", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)"},
		// Magic must be at offset zero: a leading byte defeats every entry.
		{"pe-offset", []byte{0x00, 'M', 'Z'}, ""},
		{"elf-offset", []byte{0x00, 0x7F, 'E', 'L', 'F'}, ""},
		// Only the first four bytes are examined: magic past the header
		// window must not match a whole-file scan.
		{"pe-late", []byte{0x00, 0x00, 0x00, 0x00, 'M', 'Z'}, ""},
		{"elf-late", []byte{0x00, 0x00, 0x00, 0x00, 0x7F, 'E', 'L', 'F'}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := executable.Kind(write(tc.name, tc.data))
			if tc.want == "" && ok {
				t.Errorf("executable.Kind(%s) = %q, want no match", tc.name, got)
			}
			if tc.want != "" && (!ok || got != tc.want) {
				t.Errorf("executable.Kind(%s) = (%q, %v), want (%q, true)", tc.name, got, ok, tc.want)
			}
		})
	}
}

func newAuditRepository(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	// Keep the fixture independent of any repository selected by the caller.
	t.Setenv("GIT_DIR", filepath.Join(dir, ".git"))
	t.Setenv("GIT_COMMON_DIR", filepath.Join(dir, ".git"))
	t.Setenv("GIT_WORK_TREE", dir)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(dir, ".git", "index"))
	auditGit(t, "init", "--quiet")
}

func auditGit(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.CommandContext(t.Context(), "git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestTrackedFiles_IndexSelection(t *testing.T) {
	cases := []struct {
		name    string
		tracked []string
	}{
		{name: "empty index"},
		{name: "names preserved without quoting", tracked: []string{"-leading.txt", "file with spaces.txt", "unicode-λ.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newAuditRepository(t)
			for _, name := range tc.tracked {
				if err := os.WriteFile(name, []byte("source\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				auditGit(t, "add", "--", name)
			}
			for name, data := range map[string][]byte{
				".gitignore":      []byte("ignored-build\n"),
				"ignored-build":   {0x7f, 'E', 'L', 'F'},
				"untracked-build": {'M', 'Z'},
			} {
				if err := os.WriteFile(name, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := trackedFiles()
			if err != nil {
				t.Fatalf("trackedFiles(): %v", err)
			}
			if diff := cmp.Diff(tc.tracked, got); diff != "" {
				t.Errorf("trackedFiles() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTrackedFiles_PreservesGitFailure(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing.git"))
	files, err := trackedFiles()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("trackedFiles() error = %v, want wrapped *exec.ExitError", err)
	}
	if files != nil {
		t.Errorf("trackedFiles() = %q on git failure, want nil", files)
	}
}

func TestMain_IgnoresUntrackedExecutables(t *testing.T) {
	newAuditRepository(t)
	for name, data := range map[string][]byte{
		"source.go":       []byte("package main\n"),
		"fixture.png":     {0x89, 'P', 'N', 'G'},
		"untracked-build": {'M', 'Z'},
	} {
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	auditGit(t, "add", "--", "source.go", "fixture.png")
	out, err := os.CreateTemp(t.TempDir(), "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = out
	t.Cleanup(func() {
		os.Stdout = original
		if err := out.Close(); err != nil {
			t.Errorf("close captured stdout: %v", err)
		}
	})
	main()
	got, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "audit_committed_binaries: 2 tracked files, no compiled executables\n"
	if string(got) != want {
		t.Errorf("main() stdout = %q, want %q", got, want)
	}
}

func TestMain_EmptyIndexReportsZero(t *testing.T) {
	newAuditRepository(t)
	out, err := os.CreateTemp(t.TempDir(), "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = out
	t.Cleanup(func() {
		os.Stdout = original
		if err := out.Close(); err != nil {
			t.Errorf("close captured stdout: %v", err)
		}
	})
	main()
	got, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "audit_committed_binaries: 0 tracked files, no compiled executables\n"
	if string(got) != want {
		t.Errorf("main() stdout = %q, want %q", got, want)
	}
}

func TestMain_TrackedMinimalPeViaCallerIsRejected(t *testing.T) {
	const childKey = "CODENERD_AUDIT_BINARIES_TEST_CHILD_MINIMAL"
	if os.Getenv(childKey) == "1" {
		main()
		return
	}
	newAuditRepository(t)
	if err := os.WriteFile("tiny-build", []byte{'M', 'Z'}, 0o600); err != nil {
		t.Fatal(err)
	}
	auditGit(t, "add", "--", "tiny-build")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestMain_TrackedMinimalPeViaCallerIsRejected$")
	cmd.Env = append(os.Environ(), childKey+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("main() error = %v, want exit code 1; stderr: %s", err, &stderr)
	}
	if got := exitErr.ExitCode(); got != 1 {
		t.Errorf("main() exit code = %d, want 1; stderr: %s", got, &stderr)
	}
	for _, want := range []string{"1 compiled executable(s) tracked in git:", "tiny-build", "PE (Windows)"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("main() stderr = %q, want diagnostic %q", stderr.String(), want)
		}
	}
}

func TestMain_ExitCodes(t *testing.T) {
	const childKey = "CODENERD_AUDIT_BINARIES_TEST_CHILD"
	if os.Getenv(childKey) == "1" {
		main()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		gitFailure   bool
		wantCode     int
		wantMessages []string
	}{
		{
			name:     "tracked executables rejected together",
			wantCode: 1,
			wantMessages: []string{
				"2 compiled executable(s) tracked in git:",
				"  linux-build  (ELF (Linux), 0.0 MB)",
				"  windows-build  (PE (Windows), 0.0 MB)",
				"Use `go run ./cmd/tools/foo` instead",
			},
		},
		{
			name:         "git failure is distinct from a finding",
			gitFailure:   true,
			wantCode:     2,
			wantMessages: []string{"audit_committed_binaries: git ls-files:"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newAuditRepository(t)
			if tc.gitFailure {
				t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing.git"))
			} else {
				for name, data := range map[string][]byte{
					"linux-build":   {0x7f, 'E', 'L', 'F'},
					"windows-build": {'M', 'Z'},
				} {
					if err := os.WriteFile(name, data, 0o600); err != nil {
						t.Fatal(err)
					}
					auditGit(t, "add", "--", name)
				}
			}
			cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestMain_ExitCodes$")
			cmd.Env = append(os.Environ(), childKey+"=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("main() error = %v, want exit code %d; stderr: %s", err, tc.wantCode, &stderr)
			}
			if got := exitErr.ExitCode(); got != tc.wantCode {
				t.Errorf("main() exit code = %d, want %d; stderr: %s", got, tc.wantCode, &stderr)
			}
			if stdout.Len() != 0 {
				t.Errorf("main() stdout = %q, want empty on failure", stdout.String())
			}
			for _, message := range tc.wantMessages {
				if !strings.Contains(stderr.String(), message) {
					t.Errorf("main() stderr = %q, want diagnostic %q", stderr.String(), message)
				}
			}
		})
	}
}

func TestExecutableKind_OpenErrorStaysSilent(t *testing.T) {
	missing := "/nonexistent-dir-for-audit-test/nope"
	if kind, ok := executable.Kind(missing); ok {
		t.Errorf("executable.Kind(missing) = %q, want no match", kind)
	}
	brokenLink := "/tmp/definitely-missing-audit-link-xyz"
	if kind, ok := executable.Kind(brokenLink); ok {
		t.Errorf("executable.Kind(broken) = %q, want no match", kind)
	}
}
