package shell

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSearchFoundNothing(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		exitCode int
		stderr   string
		want     bool
	}{
		// The verified defect: grep exits 1 when it matched nothing.
		{"grep no match", "echo hello | grep nomatch", 1, "", true},
		{"rg no match", "rg TODO internal", 1, "", true},
		{"findstr no match", "type x.txt | findstr foo", 1, "", true},
		{"last stage is what counts", "cat f | sed s/a/b/ | grep zzz", 1, "", true},

		// Real failures must stay failures.
		{"grep exit 2 is a real error", "grep -r pat /nope", 2, "", false},
		{"stderr means something broke", "grep pat f", 1, "grep: f: No such file", false},
		{"go build failing exits 1", "go build ./...", 1, "", false},
		{"go test failing exits 1", "go test ./... | tee out", 1, "", false},
		{"non-exit error", "grep pat f", -1, "", false},
		{"success is not routed here", "grep pat f", 0, "", false},

		// grep upstream of a non-search final stage: the shell reports the LAST
		// stage's code, so a 1 here belongs to that stage, not to grep.
		{"grep is not the last stage", "grep pat f | wc -l", 1, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchFoundNothing(tc.command, tc.exitCode, tc.stderr); got != tc.want {
				t.Fatalf("searchFoundNothing(%q, %d, %q) = %v, want %v",
					tc.command, tc.exitCode, tc.stderr, got, tc.want)
			}
		})
	}
}

// TestRunCommandNoMatchIsNotAFailure is the end-to-end proof. Before the fix
// this exact call returned `command failed: exit status 1` with empty output,
// so a search that correctly found nothing was indistinguishable from broken
// tooling.
func TestRunCommandNoMatchIsNotAFailure(t *testing.T) {
	out, err := executeRunCommand(context.Background(), map[string]any{
		"command":         "echo hello | grep definitely-not-present-anywhere",
		"timeout_seconds": 30,
	})
	if err != nil {
		t.Fatalf("a search with no matches was reported as a failure: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "no matches") {
		t.Errorf("output should say there were no matches; got %q", out)
	}
}

// TestRunCommandRealFailureStillFails is the guard on the other side: the fix
// must not swallow genuine non-zero exits.
func TestRunCommandRealFailureStillFails(t *testing.T) {
	_, err := executeRunCommand(context.Background(), map[string]any{
		"command":         "echo hello | grep -r pattern /no/such/path/at/all",
		"timeout_seconds": 30,
	})
	if err == nil {
		t.Fatal("a genuine grep error was swallowed as a no-match")
	}
}

func TestCommandStagesRespectsQuoting(t *testing.T) {
	cases := []struct {
		command string
		want    []string
	}{
		{`echo "a | b" | grep x`, []string{`echo "a | b"`, "grep x"}},
		{`echo 'a ; b' ; grep x`, []string{`echo 'a ; b'`, "grep x"}},

		// Escaped quotes. Before the two parsers were unified, the stage
		// splitter treated the backslash-escaped quote as closing the string
		// and split at the pipe inside it, yielding a final stage of `b"`.
		{`echo "a \" | b" | grep x`, []string{`echo "a \" | b"`, "grep x"}},
		{"echo \"a `\" | b\" | grep x", []string{"echo \"a `\" | b\"", "grep x"}},
	}
	for _, tc := range cases {
		if got := commandStages(tc.command); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("commandStages(%q) = %#v, want %#v", tc.command, got, tc.want)
		}
	}
}

// TestParsersAgreeOnQuoting is the structural guard behind the unification:
// whenever commandStages finds more than one stage the command must also be
// compound, because both answers now come from the same scan. If these ever
// disagree, a command runs through the wrong execution path or its exit code is
// attributed to the wrong stage.
func TestParsersAgreeOnQuoting(t *testing.T) {
	commands := []string{
		`echo "a \" | b" | grep x`,
		"echo `\"` | grep x",
		`echo 'a | b'`,
		`echo "a | b"`,
		"go test ./... | grep FAIL",
		"go build ./...",
		`grep "a|b" file`,
		`echo "unterminated | grep x`,
	}
	for _, cmd := range commands {
		multi := len(commandStages(cmd)) > 1
		if multi && !isCompoundCommand(cmd) {
			t.Errorf("%q splits into %d stages but isCompoundCommand says simple",
				cmd, len(commandStages(cmd)))
		}
	}
}

func TestStageBinary(t *testing.T) {
	cases := map[string]string{
		"grep -n foo":            "grep",
		"  head   -5  ":          "head",
		"CGO_ENABLED=1 go build": "go",
		"/usr/bin/wc -l":         "wc",
		`C:\tools\grep.exe x`:    "grep",
		`"grep" -n foo`:          "grep",
		`'grep' -n foo`:          "grep",
		"":                       "",
	}
	for in, want := range cases {
		if got := stageBinary(in); got != want {
			t.Errorf("stageBinary(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRunCommandDescriptionNamesTheHost guards the second half of the fix: the
// description must commit to one host, because the previous text named both
// Windows and Unix routing and so told the model nothing about which it had —
// and nothing else in the prompt carries the OS either.
func TestRunCommandDescriptionNamesTheHost(t *testing.T) {
	desc := runCommandDescription()
	if !strings.Contains(desc, "This host is") {
		t.Fatalf("description does not state the host: %q", desc)
	}
	if strings.Contains(desc, "on Windows, sh -c elsewhere") {
		t.Fatal("description still describes both hosts unconditionally")
	}
}
