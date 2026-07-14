package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// runBuiltinFallback serves a small set of ubiquitous, read-only shell commands
// with cross-platform Go implementations. It is invoked by executeRunCommand
// ONLY when the requested binary is not found on PATH — so a real installed tool
// always takes precedence and behavior is unchanged on systems that have these
// commands.
//
// Why this exists (F-CMD-2 / contract #4): campaign checkpoint reviewers and
// shards habitually reach for unix coreutils (rg, grep, ls, wc, cat, head,
// tail) to inspect the tree. On Windows those binaries are absent, so
// run_command hard-failed ("exec: rg: executable file not found") and the
// /shard_validation checkpoint could never complete its analysis — it exhausted
// its retries and advanced UNVERIFIED (observed live 2026-07-13, campaigns
// e6f9b0eb and 70bdd472). Serving real results for these read-only commands lets
// verification actually run on any platform.
//
// It returns (output, true) when it handled the command, or ("", false) to let
// the caller fall through to the normal exec path (which preserves the prior
// not-found error for genuinely unavailable commands). All builtins are
// strictly read-only.
func runBuiltinFallback(argv []string, workingDir string) (string, bool) {
	if len(argv) == 0 {
		return "", false
	}
	name := strings.ToLower(filepath.Base(argv[0]))
	// Strip a Windows .exe suffix the model may append.
	name = strings.TrimSuffix(name, ".exe")
	args := argv[1:]

	switch name {
	case "pwd":
		return builtinPwd(workingDir), true
	case "echo":
		return strings.Join(args, " "), true
	case "ls", "dir":
		return builtinLs(args, workingDir), true
	case "cat":
		return builtinCat(args, workingDir), true
	case "head":
		return builtinHeadTail(args, workingDir, true), true
	case "tail":
		return builtinHeadTail(args, workingDir, false), true
	case "wc":
		return builtinWc(args, workingDir), true
	case "grep", "rg", "egrep", "fgrep":
		// rg defaults to recursive; grep only recurses with -r/-R. We honor an
		// explicit path and recurse into directories in both cases, which
		// matches how the model uses them for code search.
		return builtinGrep(name, args, workingDir), true
	default:
		return "", false
	}
}

// resolvePath joins a possibly-relative path against the command working dir so
// builtins behave like a shell running in that directory.
func resolvePath(p, workingDir string) string {
	if p == "" {
		p = "."
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	base := workingDir
	if base == "" {
		if wd, err := os.Getwd(); err == nil {
			base = wd
		}
	}
	return filepath.Clean(filepath.Join(base, p))
}

// splitFlags separates leading -flags from positional operands. Values that
// look like combined short flags (e.g. -ln) are returned as a set of runes.
func splitFlags(args []string) (flags map[rune]bool, flagVals map[string]string, operands []string) {
	flags = map[rune]bool{}
	flagVals = map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			operands = append(operands, args[i+1:]...)
			return
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// -n <num> style (consume next arg for n/A/B/C)
			body := strings.TrimLeft(a, "-")
			if (body == "n" || body == "A" || body == "B" || body == "C" || body == "m") && i+1 < len(args) {
				flagVals[body] = args[i+1]
				i++
				continue
			}
			// -n5 style
			if len(body) > 1 && (body[0] == 'n' || body[0] == 'm') {
				if _, err := strconv.Atoi(body[1:]); err == nil {
					flagVals[string(body[0])] = body[1:]
					continue
				}
			}
			for _, r := range body {
				flags[r] = true
			}
		default:
			operands = append(operands, a)
		}
	}
	return
}

func builtinPwd(workingDir string) string {
	if workingDir != "" {
		abs, err := filepath.Abs(workingDir)
		if err == nil {
			return abs
		}
		return workingDir
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func builtinLs(args []string, workingDir string) string {
	flags, _, operands := splitFlags(args)
	if len(operands) == 0 {
		operands = []string{"."}
	}
	showAll := flags['a']
	var out strings.Builder
	multi := len(operands) > 1
	for idx, target := range operands {
		full := resolvePath(target, workingDir)
		info, err := os.Stat(full)
		if err != nil {
			fmt.Fprintf(&out, "ls: cannot access '%s': %v\n", target, err)
			continue
		}
		if !info.IsDir() {
			out.WriteString(target + "\n")
			continue
		}
		if multi {
			if idx > 0 {
				out.WriteString("\n")
			}
			out.WriteString(target + ":\n")
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			fmt.Fprintf(&out, "ls: cannot open directory '%s': %v\n", target, err)
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !showAll && strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, n := range names {
			out.WriteString(n + "\n")
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

func builtinCat(args []string, workingDir string) string {
	_, _, operands := splitFlags(args)
	if len(operands) == 0 {
		return ""
	}
	var out strings.Builder
	for _, f := range operands {
		data, err := os.ReadFile(resolvePath(f, workingDir))
		if err != nil {
			fmt.Fprintf(&out, "cat: %s: %v\n", f, err)
			continue
		}
		out.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

func builtinHeadTail(args []string, workingDir string, head bool) string {
	flags, flagVals, operands := splitFlags(args)
	_ = flags
	n := 10
	if v, ok := flagVals["n"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			n = parsed
		}
	}
	if len(operands) == 0 {
		return ""
	}
	var out strings.Builder
	for _, f := range operands {
		data, err := os.ReadFile(resolvePath(f, workingDir))
		if err != nil {
			fmt.Fprintf(&out, "%s: %s: %v\n", map[bool]string{true: "head", false: "tail"}[head], f, err)
			continue
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if head {
			if n < len(lines) {
				lines = lines[:n]
			}
		} else {
			if n < len(lines) {
				lines = lines[len(lines)-n:]
			}
		}
		out.WriteString(strings.Join(lines, "\n"))
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func builtinWc(args []string, workingDir string) string {
	flags, _, operands := splitFlags(args)
	onlyLines := flags['l'] && !flags['w'] && !flags['c']
	onlyWords := flags['w'] && !flags['l'] && !flags['c']
	onlyBytes := flags['c'] && !flags['l'] && !flags['w']
	if len(operands) == 0 {
		return "wc: no input files"
	}
	var out strings.Builder
	for _, f := range operands {
		data, err := os.ReadFile(resolvePath(f, workingDir))
		if err != nil {
			fmt.Fprintf(&out, "wc: %s: %v\n", f, err)
			continue
		}
		lineCount := strings.Count(string(data), "\n")
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lineCount++ // count a final line without trailing newline
		}
		wordCount := len(strings.Fields(string(data)))
		byteCount := len(data)
		switch {
		case onlyLines:
			fmt.Fprintf(&out, "%d %s\n", lineCount, f)
		case onlyWords:
			fmt.Fprintf(&out, "%d %s\n", wordCount, f)
		case onlyBytes:
			fmt.Fprintf(&out, "%d %s\n", byteCount, f)
		default:
			fmt.Fprintf(&out, "%d %d %d %s\n", lineCount, wordCount, byteCount, f)
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

// builtinGrep implements a read-only grep/rg over files and directories.
func builtinGrep(name string, args []string, workingDir string) string {
	flags, flagVals, operands := splitFlags(args)
	if len(operands) == 0 {
		return fmt.Sprintf("%s: no pattern given", name)
	}
	patternStr := operands[0]
	paths := operands[1:]
	if len(paths) == 0 {
		paths = []string{"."} // rg/grep -r style: default to cwd
	}

	ignoreCase := flags['i']
	showLineNum := flags['n'] || name == "rg" // rg shows line numbers by default
	filesOnly := flags['l']
	maxCount := 0
	if v, ok := flagVals["m"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil {
			maxCount = parsed
		}
	}

	reStr := patternStr
	if ignoreCase {
		reStr = "(?i)" + reStr
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		// Fall back to a literal match if the pattern isn't a valid regex.
		lit := regexp.QuoteMeta(patternStr)
		if ignoreCase {
			lit = "(?i)" + lit
		}
		re, err = regexp.Compile(lit)
		if err != nil {
			return fmt.Sprintf("%s: invalid pattern: %v", name, patternStr)
		}
	}

	var out strings.Builder
	matches := 0
	const maxMatches = 2000 // safety bound on output volume

	searchFile := func(path string) {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		rel := path
		if r, err := filepath.Rel(resolvePath(".", workingDir), path); err == nil {
			rel = filepath.ToSlash(r)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		lineNo := 0
		fileHits := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				if filesOnly {
					out.WriteString(rel + "\n")
					return
				}
				if showLineNum {
					fmt.Fprintf(&out, "%s:%d:%s\n", rel, lineNo, line)
				} else {
					fmt.Fprintf(&out, "%s:%s\n", rel, line)
				}
				matches++
				fileHits++
				if matches >= maxMatches {
					out.WriteString("...[truncated]\n")
					return
				}
				if maxCount > 0 && fileHits >= maxCount {
					return
				}
			}
		}
	}

	for _, p := range paths {
		full := resolvePath(p, workingDir)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			_ = filepath.WalkDir(full, func(walkPath string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					// Skip noise directories the model never wants to grep.
					base := d.Name()
					if base == ".git" || base == "node_modules" || base == "vendor" {
						return filepath.SkipDir
					}
					return nil
				}
				searchFile(walkPath)
				if matches >= maxMatches {
					return filepath.SkipAll
				}
				return nil
			})
		} else {
			searchFile(full)
		}
		if matches >= maxMatches {
			break
		}
	}

	res := strings.TrimRight(out.String(), "\n")
	if res == "" {
		// grep/rg convention: no matches => empty output (exit 1), but tools
		// here return strings; an explicit note is friendlier to the LLM.
		return ""
	}
	return res
}
