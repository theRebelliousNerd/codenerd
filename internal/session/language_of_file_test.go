package session

import (
	"testing"

	"codenerd/internal/core"
)

// realKernel is the production kernel over the embedded policy corpus, for
// tests whose subject is a decision the policy makes.
func realKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// The language a compile runs under is the kernel's answer
// (policy/coder_language.mg), not a Go switch. A document gets a prose key so
// the language-tagged code corpus is excluded from its turn; a file the policy
// has no row for keeps the project's language (""). The measurement is
// transient: nothing about the asked-about file is left in the kernel.
func TestLanguageOfFile_TheKernelNamesTheLanguage(t *testing.T) {
	k := realKernel(t)
	e := NewExecutor(k, nil, nil, nil, nil, nil)

	cases := map[string]string{
		"Docs/architecture/features/01-VISION.md": "/markdown",
		"README.MD":                       "/markdown",
		"notes.markdown":                  "/markdown",
		"internal/context/working_set.mg": "/mangle",
		"internal/session/executor.go":    "/go",
		"web/app.mjs":                     "/javascript",
		"scripts/tool.py":                 "/python",
		"Makefile":                        "",
		"assets/logo.svg":                 "",
	}
	for path, want := range cases {
		if got := e.languageOfFile(path); got != want {
			t.Errorf("languageOfFile(%q) = %q, want %q", path, got, want)
		}
	}

	left, err := k.Query("file_extension")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("the extension measurement outlived the question: %v", left)
	}
}
