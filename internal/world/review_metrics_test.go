package world

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
)

const reviewFixture = `package svc

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
)

const apiKey = "sk_live_0123456789abcdef"

func ConstantQuery(db *sql.DB) { db.Exec("DELETE FROM t") }

func ConcatQuery(db *sql.DB, name string) { db.Query("SELECT * FROM t WHERE n = '" + name + "'") }

func SprintfQuery(db *sql.DB, id string) { db.QueryRow(fmt.Sprintf("SELECT * FROM t WHERE id = %s", id)) }

func LiteralCommand() { exec.Command("ls", "-l") }

func ParamCommand(arg string) { exec.Command("sh", "-c", arg) }

func EnvCommand() { exec.Command(os.Getenv("SHELL")) }

func Hash(b []byte) [16]byte { return md5.Sum(b) }

func Branchy(a, b int) int {
	if a > 0 && b > 0 {
		for i := 0; i < a; i++ {
			switch {
			case i == 1:
				return 1
			case i == 2:
				return 2
			}
		}
	}
	return 0
}
`

func flowRows(facts []core.Fact) map[string]string {
	rows := map[string]string{}
	for _, f := range facts {
		if f.Predicate == "detected_security_flow" {
			rows[fmt.Sprint(f.Args[2])+"@"+fmt.Sprint(f.Args[1])] = fmt.Sprint(f.Args[3]) + ":" + fmt.Sprint(f.Args[4])
		}
	}
	return rows
}

func TestGoReviewFacts_SecurityFlowsAreOnlyDynamicSinks(t *testing.T) {
	facts := goReviewFacts("svc/svc.go", []byte(reviewFixture))
	rows := flowRows(facts)
	want := map[string]string{
		"/hardcoded_secret@11": "/literal:70",
		"/sql_sink@15":         "/parameter:80",
		"/sql_sink@17":         "/parameter:80",
		"/command_sink@21":     "/parameter:80",
		"/command_sink@23":     "/env_var:80",
		"/weak_crypto@25":      "/internal:90",
	}
	for k, v := range want {
		if rows[k] != v {
			t.Errorf("flow %s = %q, want %q (all: %v)", k, rows[k], v, rows)
		}
	}
	for k := range rows {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected flow %s = %s: a constant query and a literal command are never flows", k, rows[k])
		}
	}
}

func TestGoReviewFacts_MeasuresComplexityNestingAndLines(t *testing.T) {
	facts := goReviewFacts("svc/svc.go", []byte(reviewFixture))
	got := map[string]string{}
	for _, f := range facts {
		switch f.Predicate {
		case "cyclomatic_complexity":
			got["cc:"+fmt.Sprint(f.Args[1])] = fmt.Sprint(f.Args[2])
		case "nesting_depth":
			got["depth:"+fmt.Sprint(f.Args[1])] = fmt.Sprint(f.Args[2])
		case "file_line_count":
			got["lines"] = fmt.Sprint(f.Args[1])
		}
	}
	// Branchy: 1 + if + && + for + two cases = 6; if > for > switch = depth 3.
	if got["cc:Branchy"] != "6" || got["depth:Branchy"] != "3" {
		t.Errorf("Branchy measured %v, want complexity 6 and nesting 3", got)
	}
	if got["cc:ConstantQuery"] != "1" || got["depth:ConstantQuery"] != "0" {
		t.Errorf("a straight line function is complexity 1, depth 0: %v", got)
	}
	if want := fmt.Sprint(strings.Count(reviewFixture, "\n")); got["lines"] != want {
		t.Errorf("file_line_count = %s, want %s", got["lines"], want)
	}
}

func TestGoReviewFacts_UnparseableFileStillHasALineCount(t *testing.T) {
	facts := goReviewFacts("bad.go", []byte("package x\nfunc ("))
	if len(facts) != 1 || facts[0].Predicate != "file_line_count" {
		t.Errorf("an unparseable file yields only its line count: %v", facts)
	}
}

func TestGoReviewFacts_PlaceholderSitesAreWhatTheFileSaysIsUnfinished(t *testing.T) {
	src := `package p

// TODO: handle the error case
func Pending() {}

func Stub() error { return errors.New("not implemented") }

func Real() int { return 1 }

// Close satisfies io.Closer by doing nothing, on purpose.
func (t T) Close() {}
`
	got := map[string]bool{}
	for _, f := range goReviewFacts("p/p.go", []byte(src)) {
		if f.Predicate == "placeholder_site" {
			got[fmt.Sprint(f.Args[1])+":"+fmt.Sprint(f.Args[2])] = true
		}
	}
	for _, want := range []string{"3:/todo", "4:/empty_function", "6:/not_implemented"} {
		if !got[want] {
			t.Errorf("missing placeholder site %s: %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("a real function and a deliberate no-op method are not placeholders: %v", got)
	}
}
