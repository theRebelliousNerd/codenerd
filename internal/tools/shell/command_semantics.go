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

// scanShell walks s and reports, for every byte, whether it sits in a position
// where a shell operator would be interpreted.
//
// It exists so that "is this command compound" and "what are its stages" cannot
// answer differently. They previously each carried their own quote tracking,
// and the copies had already drifted: the stage splitter did not honour a quote
// escaped with a backslash or backtick, so `echo "a \" | b" | grep x` split at
// the pipe *inside* the quoted string and the final stage came out as `b"`.
// A misread final stage is a misread exit code, which is the whole decision
// this file exists to make.
//
// quoted is true for anything the shell would treat as literal text -- bytes
// inside quotes, the quote delimiters themselves, and an escape character with
// the quote it protects -- so a caller can simply ignore operators while quoted
// is set. fn returns false to stop the scan early.
func scanShell(s string, fn func(i int, c byte, quoted bool) bool) {
	inSingle := false
	inDouble := false

	// emitPair reports an escape byte and the quote it protects, keeping the
	// escape from opening or closing a quoted region.
	emitPair := func(i *int, c byte) bool {
		if !fn(*i, c, true) {
			return false
		}
		*i++
		return fn(*i, s[*i], true)
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		escapes := c == '\\' || c == '`'

		switch {
		case inSingle:
			if escapes && i+1 < len(s) && s[i+1] == '\'' {
				if !emitPair(&i, c) {
					return
				}
				continue
			}
			if c == '\'' {
				inSingle = false
			}
			if !fn(i, c, true) {
				return
			}
		case inDouble:
			if escapes && i+1 < len(s) && s[i+1] == '"' {
				if !emitPair(&i, c) {
					return
				}
				continue
			}
			if c == '"' {
				inDouble = false
			}
			if !fn(i, c, true) {
				return
			}
		default:
			if escapes && i+1 < len(s) && (s[i+1] == '\'' || s[i+1] == '"') {
				if !emitPair(&i, c) {
					return
				}
				continue
			}
			if c == '\'' {
				inSingle = true
				if !fn(i, c, true) {
					return
				}
				continue
			}
			if c == '"' {
				inDouble = true
				if !fn(i, c, true) {
					return
				}
				continue
			}
			if !fn(i, c, false) {
				return
			}
		}
	}
}

// commandStages splits a command into its pipeline/sequence stages. Operators
// inside quotes are text, not separators.
func commandStages(s string) []string {
	var stages []string
	var cur strings.Builder

	flush := func() {
		if t := strings.TrimSpace(cur.String()); t != "" {
			stages = append(stages, t)
		}
		cur.Reset()
	}

	skipNext := false
	scanShell(s, func(i int, c byte, quoted bool) bool {
		if skipNext {
			skipNext = false
			return true
		}
		if !quoted {
			switch c {
			case '|', '&':
				// A doubled operator (|| or &&) is one separator.
				if i+1 < len(s) && s[i+1] == c {
					skipNext = true
				}
				flush()
				return true
			case ';', '\n', '\r':
				flush()
				return true
			}
		}
		cur.WriteByte(c)
		return true
	})
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
		// A quoted binary is still that binary: "grep" -n foo invokes grep.
		f = strings.Trim(f, `"'`)
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
