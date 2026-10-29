package projectdoc

import "testing"

// The audit's witness: a reader name at the end of a pipeline used to erase
// the whole stage from classification, so `echo hello | sort -o guarded.txt`
// was classified as `echo hello` (read-only) while sh -c ran the write.
// Every one of these writes a file, runs a helper, or cannot be proven not
// to; none may classify as read-only or verification.
func TestClassifyShellEffect_ReaderTailsThatWriteAreNotErased(t *testing.T) {
	for _, cmd := range []string{
		"echo hello | sort -o guarded.txt",
		"echo hello | sort --output=guarded.txt",
		"echo hello | sort --output guarded.txt",
		"echo hello | sort --out=guarded.txt", // GNU accepts unambiguous abbreviations
		"echo hello | sort -uo guarded.txt",   // bundled short options
		"echo hello | sort -oguarded.txt",
		"echo hello | sort -T /tmp",
		"echo hello | sort --compress-program=gzip",
		"go test ./... | head | sort -o guarded.txt", // behind another reader
		"go test ./... | sort -o guarded.txt | head", // before another reader
		"echo hello | uniq - guarded.txt",            // positional output
		"echo hello | rg --pre cat foo",
		"echo hello | rg -z foo",
		"echo hello | tail -f log.txt",
		"echo hello | sort $OPTS",
		`echo hello | sort "$OPTS"`,
		"echo hello | sort -*",
		"echo hello | /usr/bin/sort",
		"echo hello | sort 'unterminated",
		"false || sort -o guarded.txt",
	} {
		if got := ClassifyShellEffect(cmd); got == ShellEffectReadOnly || got == ShellEffectVerification {
			t.Errorf("ClassifyShellEffect(%q) = %s; the tail's effect was erased", cmd, got)
		}
	}
}

// The convenience the elision exists for survives: verification and
// read-only commands piped through readers whose arguments are all inside
// the reader's grammar.
func TestClassifyShellEffect_ProvenReaderTailsStillElide(t *testing.T) {
	for cmd, want := range map[string]ShellEffectKind{
		"go test ./... | head -n 20":                              ShellEffectVerification,
		"go test ./... | head -20":                                ShellEffectVerification,
		"go test ./... | tail -n 5":                               ShellEffectVerification,
		"go test ./... 2>&1 | grep -E 'FAIL|panic' | head":        ShellEffectVerification,
		`go test ./... 2>&1 | grep -v "^ok"`:                      ShellEffectVerification,
		"go test ./... | grep -v 'ok$'":                           ShellEffectVerification,
		"go vet ./... 2>&1 | sort -k2 -n | uniq -c":               ShellEffectVerification,
		"go build ./... 2>&1 | wc -l":                             ShellEffectVerification,
		"go test ./... | rg -i --max-count 3 fail":                ShellEffectVerification,
		"git log --oneline | head -n 5":                           ShellEffectReadOnly,
		"git diff | cat -n":                                       ShellEffectReadOnly,
		"echo hello | grep \"a|sort -o x\"":                       ShellEffectReadOnly, // the pipe is quoted
		"echo hello | sort --reverse --key=2 --field-separator ,": ShellEffectReadOnly,
	} {
		if got := ClassifyShellEffect(cmd); got != want {
			t.Errorf("ClassifyShellEffect(%q) = %s, want %s", cmd, got, want)
		}
	}
}

// Both gates (session checkShellEffect and the registry write guard) decide
// through ValidateShellToolInvocation; the witness is refused there.
func TestValidateShellToolInvocation_RefusesTheSortOutputWitness(t *testing.T) {
	kind, _, err := ValidateShellToolInvocation("run_command", map[string]any{"command": "echo hello | sort -o guarded.txt"})
	if err == nil {
		t.Fatalf("witness admitted as %s", kind)
	}
}
