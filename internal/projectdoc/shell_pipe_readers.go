package projectdoc

import (
	"strings"
	"unicode"
)

// A trailing pipeline stage is dropped from shell-effect classification only
// when the whole stage is proven to be an output reader: a known reader name
// AND every option and operand inside that reader's positive grammar. A reader
// name alone is not evidence: `sort -o FILE` writes FILE, `uniq IN OUT` writes
// OUT, `rg --pre CMD` runs CMD, and GNU long options accept unambiguous
// abbreviations (`--out`), so a denylist of output flags can never be complete
// (Docs/audits/shell-tail-effect-admission.md). An option that is not listed
// below keeps the stage, and the remaining pipe then fails classification
// closed.

// pipeReader is the positive grammar of one reader that may end a pipeline.
type pipeReader struct {
	// flags are short options that take no value and may be bundled (-nr).
	flags string
	// valued are short options that take a value, attached (-n5) or as the
	// next word (-n 5).
	valued string
	// long are the exact long options allowed. A value attaches with "=" or
	// is the next word for those marked true. Abbreviations are not accepted.
	long map[string]bool
	// numeric allows the historical -NUM count form (head -20).
	numeric bool
	// maxOperands bounds positional operands; -1 is unbounded. Operands of
	// these readers are input files, except uniq's second, which is an output.
	maxOperands int
}

var pipeReaders = map[string]pipeReader{
	"head": {flags: "qvz", valued: "nc", numeric: true, maxOperands: -1, long: map[string]bool{
		"--lines": true, "--bytes": true, "--quiet": false, "--silent": false, "--verbose": false, "--zero-terminated": false}},
	// tail -f/-F never terminate; they are not readers of a finished output.
	"tail": {flags: "qvz", valued: "nc", numeric: true, maxOperands: -1, long: map[string]bool{
		"--lines": true, "--bytes": true, "--quiet": false, "--silent": false, "--verbose": false, "--zero-terminated": false}},
	"wc": {flags: "clmwL", maxOperands: -1, long: map[string]bool{
		"--bytes": false, "--chars": false, "--lines": false, "--words": false, "--max-line-length": false}},
	"cat": {flags: "AbeEnstTuv", maxOperands: -1, long: map[string]bool{
		"--show-all": false, "--number-nonblank": false, "--number": false, "--squeeze-blank": false,
		"--show-tabs": false, "--show-nonprinting": false, "--show-ends": false}},
	"grep": {flags: "EFGPivwxyclLoqsbHhnTZzaIrR", valued: "efmABCdD", maxOperands: -1, long: map[string]bool{
		"--extended-regexp": false, "--fixed-strings": false, "--basic-regexp": false, "--perl-regexp": false,
		"--regexp": true, "--file": true, "--ignore-case": false, "--no-ignore-case": false, "--invert-match": false,
		"--word-regexp": false, "--line-regexp": false, "--count": false, "--color": false, "--colour": false,
		"--files-without-match": false, "--files-with-matches": false, "--max-count": true, "--only-matching": false,
		"--quiet": false, "--silent": false, "--no-messages": false, "--byte-offset": false, "--with-filename": false,
		"--no-filename": false, "--line-number": false, "--initial-tab": false, "--null": false, "--null-data": false,
		"--after-context": true, "--before-context": true, "--context": true, "--text": false, "--recursive": false,
		"--dereference-recursive": false, "--include": true, "--exclude": true, "--exclude-dir": true}},
	// rg -z/--search-zip and --pre run helper processes; neither is listed.
	"rg": {flags: "iISsvwxclLoqnNFUHa", valued: "egtTmABCM", maxOperands: -1, long: map[string]bool{
		"--ignore-case": false, "--smart-case": false, "--case-sensitive": false, "--invert-match": false,
		"--word-regexp": false, "--line-regexp": false, "--count": false, "--count-matches": false,
		"--files-with-matches": false, "--files-without-match": false, "--only-matching": false, "--quiet": false,
		"--line-number": false, "--no-line-number": false, "--fixed-strings": false, "--multiline": false,
		"--with-filename": false, "--no-filename": false, "--text": false, "--regexp": true, "--glob": true,
		"--type": true, "--type-not": true, "--max-count": true, "--after-context": true, "--before-context": true,
		"--context": true, "--max-columns": true, "--color": true, "--hidden": false, "--no-ignore": false}},
	// sort -o/--output writes, -T/--temporary-directory writes, and
	// --compress-program runs a helper; none is listed.
	"sort": {flags: "bdfgiMhnRrsuVcCz", valued: "ktS", maxOperands: -1, long: map[string]bool{
		"--ignore-leading-blanks": false, "--dictionary-order": false, "--ignore-case": false,
		"--general-numeric-sort": false, "--human-numeric-sort": false, "--month-sort": false,
		"--numeric-sort": false, "--random-sort": false, "--reverse": false, "--version-sort": false,
		"--stable": false, "--unique": false, "--zero-terminated": false, "--check": false,
		"--key": true, "--field-separator": true, "--buffer-size": true}},
	// uniq's second operand is an output file.
	"uniq": {flags: "cdDiuz", valued: "fsw", maxOperands: 1, long: map[string]bool{
		"--count": false, "--repeated": false, "--ignore-case": false, "--unique": false, "--zero-terminated": false,
		"--skip-fields": true, "--skip-chars": true, "--check-chars": true}},
}

// stripBenignOutputTail removes trailing "2>&1" and pipeline stages that are
// proven output readers, for classification only; the command that runs is
// never changed. Every removed stage is checked in full; the first stage that
// cannot be proven a reader stops the stripping, and the pipe it leaves makes
// ClassifyShellEffect fail closed.
func stripBenignOutputTail(command string) string {
	s := strings.TrimSpace(command)
	for {
		trimmed := strings.TrimSpace(s)
		if strings.HasSuffix(trimmed, "2>&1") {
			s = strings.TrimSpace(strings.TrimSuffix(trimmed, "2>&1"))
			continue
		}
		head, tail, ok := splitLastUnquotedPipe(trimmed)
		if !ok || !isOutputReaderStage(tail) {
			break
		}
		s = head
	}
	return s
}

// splitLastUnquotedPipe splits at the last pipe the shell would see, not at
// a "|" inside a quoted argument (grep "a|b").
func splitLastUnquotedPipe(command string) (head, tail string, ok bool) {
	masked, balanced := maskQuotedSegments(command)
	if !balanced {
		return "", "", false
	}
	rs, ms := []rune(command), []rune(masked)
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i] != '|' {
			continue
		}
		if i > 0 && ms[i-1] == '|' {
			return "", "", false // "||" is control flow, not a pipe
		}
		return strings.TrimSpace(string(rs[:i])), strings.TrimSpace(string(rs[i+1:])), true
	}
	return "", "", false
}

// isOutputReaderStage reports whether one pipeline stage is a known reader
// whose every word is inside its grammar.
func isOutputReaderStage(stage string) bool {
	if stage == "" {
		return false
	}
	// Expansion can turn any word into an option the grammar never saw:
	// command substitution, variables (sort $OPTS), globs that match a file
	// named "-o..." and redirections are all refused outright when the shell
	// would act on them. Single-quoted text is literal (grep 'err$').
	if strings.ContainsAny(stage, "\r\n") {
		return false
	}
	masked, balanced := maskQuotedSegments(stage)
	if !balanced || strings.ContainsAny(masked, ";&><`$*?[{~\\") {
		return false
	}
	words, ok := shellWords(stage)
	if !ok || len(words) == 0 {
		return false
	}
	reader, known := pipeReaders[strings.ToLower(words[0])]
	if !known {
		return false
	}
	return reader.admits(words[1:])
}

func (r pipeReader) admits(args []string) bool {
	operands := 0
	optionsDone := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case optionsDone || a == "-" || !strings.HasPrefix(a, "-"):
			operands++
		case a == "--":
			optionsDone = true
		case strings.HasPrefix(a, "--"):
			name, _, hasValue := strings.Cut(a, "=")
			takesValue, listed := r.long[name]
			if !listed || (hasValue && !takesValue) {
				return false
			}
			if takesValue && !hasValue {
				i++
				if i >= len(args) {
					return false
				}
			}
		case r.numeric && isDigits(a[1:]):
		default:
			consumed, ok := r.shortCluster(a[1:])
			if !ok {
				return false
			}
			if consumed {
				i++
				if i >= len(args) {
					return false
				}
			}
		}
	}
	return r.maxOperands < 0 || operands <= r.maxOperands
}

// shortCluster checks a bundle of short options. It reports whether the
// bundle ends in a valued option whose value is the next word.
func (r pipeReader) shortCluster(cluster string) (valueIsNextWord, ok bool) {
	for j, c := range cluster {
		switch {
		case strings.ContainsRune(r.valued, c):
			return j+1 == len(cluster), true
		case strings.ContainsRune(r.flags, c):
		default:
			return false, false
		}
	}
	return false, cluster != ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// shellWords splits a stage into words the way a POSIX shell does for the
// quoting it can contain here (single and double quotes, backslash escapes
// inside double quotes). Unquoted expansion characters are refused by the
// caller; "$" and "`" inside double quotes are refused here.
func shellWords(stage string) ([]string, bool) {
	var words []string
	var cur strings.Builder
	inWord, inSingle, inDouble := false, false, false
	rs := []rune(stage)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(c)
			}
		case inDouble:
			switch {
			case c == '"':
				inDouble = false
			case c == '$' || c == '`':
				return nil, false // expanded inside double quotes
			case c == '\\' && i+1 < len(rs):
				i++
				cur.WriteRune(rs[i])
			default:
				cur.WriteRune(c)
			}
		case c == '\'':
			inSingle, inWord = true, true
		case c == '"':
			inDouble, inWord = true, true
		case unicode.IsSpace(c):
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(c)
			inWord = true
		}
	}
	if inSingle || inDouble {
		return nil, false
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, true
}
