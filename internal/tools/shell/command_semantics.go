package shell

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// runCommandDescription describes run_command in terms of the host actually
// running it. The previous static text named both the Windows and the Unix
// routing unconditionally, so it never told the model which one applied — and
// nothing else in the prompt carries the OS either, so on Windows the model was
// never informed it was on Windows. That is why it kept emitting POSIX
// pipelines into PowerShell.
func runCommandDescription() string {
	var host string
	if runtime.GOOS == "windows" {
		host = "This host is Windows. Compound commands (&&, ||, |, ;, newline, <, >) run through " +
			"PowerShell (pwsh or powershell, -NoProfile -NonInteractive -Command). PowerShell has no " +
			"grep, head, tail, wc, sed, awk, cut, tr, uniq or xargs. Prefer PowerShell equivalents " +
			"(Select-String, Select-Object -First/-Last, Measure-Object). A pipeline that does use " +
			"those POSIX utilities is routed to a POSIX shell automatically when one is installed, " +
			"but relying on that is slower and less predictable than writing for the host shell."
	} else {
		host = "This host is " + runtime.GOOS + ". Compound commands (&&, ||, |, ;, newline, <, >) " +
			"run through sh -c, so ordinary POSIX pipelines work."
	}
	return "Execute a shell command and return its output. " + host +
		" Simple commands execute directly via exec, with no shell. Operators inside single or " +
		"double quotes do not trigger compound routing. Timeout, working directory, env, output " +
		"bounds, and upstream permission decisions are preserved on every path."
}

// searchUtilities exit non-zero to report "found nothing", not "I broke".
// grep and ripgrep both use exit 1 for no-match and reserve exit 2 and above
// for real errors such as an unreadable file or a bad pattern; findstr uses 1
// the same way. That convention is what makes a no-match distinguishable from
// a genuine failure, so the set is restricted to tools known to follow it.
var searchUtilities = map[string]bool{
	"grep":    true,
	"egrep":   true,
	"fgrep":   true,
	"rg":      true,
	"ug":      true,
	"ack":     true,
	"findstr": true,
}

// searchFoundNothing reports whether a non-zero exit is a search utility saying
// "no matches" rather than a failure.
//
// This is the difference between telling the model "your search returned no
// results" and telling it "your command failed", and the second answer is both
// false and expensive: the model concludes its tooling is broken and spends
// turns re-running or working around a search that in fact worked perfectly.
//
// Only the final stage is consulted, because that is the process whose exit
// code the shell reports. Only exit code 1 qualifies: 2 and above stay
// failures, so an unreadable path or a malformed pattern is still surfaced.
func searchFoundNothing(command string, exitCode int, stderr string) bool {
	if exitCode != 1 {
		return false
	}
	if strings.TrimSpace(stderr) != "" {
		// A real diagnostic accompanies real failures; a no-match is silent.
		return false
	}
	stages := commandStages(command)
	if len(stages) == 0 {
		return false
	}
	return searchUtilities[stageBinary(stages[len(stages)-1])]
}

// commandStages splits a command into its pipeline/sequence stages, honouring
// quoting with the same rules as isCompoundCommand so an operator inside a
// quoted string is not treated as a separator.
func commandStages(s string) []string {
	var stages []string
	var cur strings.Builder
	inSingle := false
	inDouble := false

	flush := func() {
		if t := strings.TrimSpace(cur.String()); t != "" {
			stages = append(stages, t)
		}
		cur.Reset()
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			cur.WriteByte(c)
			continue
		}
		if inDouble {
			if c == '"' {
				inDouble = false
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\'':
			inSingle = true
			cur.WriteByte(c)
			continue
		case '"':
			inDouble = true
			cur.WriteByte(c)
			continue
		case '|', '&':
			// Consume a doubled operator (|| or &&) as one separator.
			if i+1 < len(s) && s[i+1] == c {
				i++
			}
			flush()
			continue
		case ';', '\n', '\r':
			flush()
			continue
		}
		cur.WriteByte(c)
	}
	flush()
	return stages
}

// stageBinary returns the command name a stage invokes, ignoring leading
// VAR=value assignments and any redirection tail. It returns "" when the stage
// has no identifiable command.
func stageBinary(stage string) string {
	fields := strings.Fields(stage)
	for _, f := range fields {
		// Skip env assignments such as CGO_ENABLED=1.
		if i := strings.IndexByte(f, '='); i > 0 && !strings.ContainsAny(f[:i], "/\\.\"'") {
			continue
		}
		f = strings.TrimPrefix(f, "(")
		if f == "" {
			continue
		}
		// Reduce a path to its final element so /usr/bin/grep still matches.
		if i := strings.LastIndexAny(f, "/\\"); i >= 0 {
			f = f[i+1:]
		}
		return strings.TrimSuffix(strings.ToLower(f), ".exe")
	}
	return ""
}

// exitCodeOf returns the process exit code carried by err, or -1 when err is
// not an exit status.
func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
