package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// nerd logs parses the log files the logger actually writes.
//
// It used to match this pattern:
//
//	^\d{2}:\d{2}:\d{2}\.\d{3}\s+\[([^\]]+)\]\s+\[(WARN|ERROR)\]\s+(.*)$
//
// against a documented example of "15:04:05.000 [api] [INFO] message". The
// logger emits none of that. Real lines look like:
//
//	2026/08/08 02:45:02.461969 [ERROR] LLM call failed: ...
//
// — a full date, six fractional digits, and no category bracket at all. The
// regex therefore matched nothing, ever, and the command reported "No warnings
// or errors found in logs." while the kernel log alone held 2,106 ERROR and
// WARN lines. Same family as the glassbox and bare-`nerd logic` defects: a
// diagnostic whose only possible output was "nothing", phrased as a fact about
// the system rather than about itself.
//
// The category was never in the line; it is the filename
// (2026-08-08_kernel.log -> kernel), which the old code already had in hand.

// logLinePattern matches the logger's real output. The date and the fractional
// part are both optional so a format tweak degrades to fewer matches rather
// than none, and the fraction accepts any digit count instead of exactly three.
var logLinePattern = regexp.MustCompile(`^(?:\d{4}/\d{2}/\d{2}\s+)?\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+\[(WARN|ERROR)\]\s+(.*)$`)

// legacyLogLinePattern matches the category-bracket form the old regex
// expected, in case any writer still produces it.
var legacyLogLinePattern = regexp.MustCompile(`^(?:\d{4}/\d{2}/\d{2}\s+)?\d{2}:\d{2}:\d{2}(?:\.\d+)?\s+\[([^\]]+)\]\s+\[(WARN|ERROR)\]\s+(.*)$`)

// maxEntriesPerCategory caps output per category. The kernel log can carry
// thousands of repeats of one line; printing all of them buries every other
// category. The dropped count is always reported — a silent cap would recreate
// the defect this command exists to fix.
const maxEntriesPerCategory = 15

// categoryFromLogFile derives the log category from the filename, stripping the
// YYYY-MM-DD_ prefix and the .log suffix.
func categoryFromLogFile(name string) string {
	base := strings.TrimSuffix(name, ".log")
	if idx := strings.Index(base, "_"); idx >= 0 && idx == 10 {
		base = base[idx+1:]
	}
	if base == "" {
		return "unknown"
	}
	return base
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Aggregate and view system errors and warnings",
	Long:  "Scans the .nerd/logs directory for [WARN] and [ERROR] entries and groups them by category.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Honour --workspace. This was hardcoded to a relative ".nerd/logs",
		// so `nerd -w <dir> logs` read the current directory's logs or failed
		// outright.
		root := workspace
		if root == "" {
			root = "."
		}
		logsDir := filepath.Join(root, ".nerd", "logs")

		files, err := os.ReadDir(logsDir)
		if err != nil {
			return fmt.Errorf("failed to read logs directory %s: %w", logsDir, err)
		}

		warnings := make(map[string][]string)
		errorsMap := make(map[string][]string)
		scanned := 0

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".log") {
				continue
			}

			path := filepath.Join(logsDir, f.Name())
			file, err := os.Open(path)
			if err != nil {
				fmt.Printf("Warning: could not open log file %s: %v\n", path, err)
				continue
			}
			scanned++

			category := categoryFromLogFile(f.Name())

			scanner := bufio.NewScanner(file)
			// Log lines can carry embedded prompts and stack traces well past
			// the 64 KB default, and a too-long line aborts the scan silently.
			scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

			for scanner.Scan() {
				line := scanner.Text()

				level, msg, cat := "", "", category
				if m := logLinePattern.FindStringSubmatch(line); len(m) == 3 {
					level, msg = m[1], m[2]
				} else if m := legacyLogLinePattern.FindStringSubmatch(line); len(m) == 4 {
					cat, level, msg = m[1], m[2], m[3]
				} else {
					continue
				}

				if level == "WARN" {
					warnings[cat] = append(warnings[cat], msg)
				} else {
					errorsMap[cat] = append(errorsMap[cat], msg)
				}
			}
			file.Close()
		}

		printLogCategory := func(title string, data map[string][]string) {
			if len(data) == 0 {
				return
			}

			total := 0
			for _, entries := range data {
				total += len(entries)
			}
			fmt.Printf("\n=== %s (%d total) ===\n", title, total)

			categories := make([]string, 0, len(data))
			for c := range data {
				categories = append(categories, c)
			}
			// Noisiest category first: that is where triage starts.
			sort.Slice(categories, func(i, j int) bool {
				if len(data[categories[i]]) != len(data[categories[j]]) {
					return len(data[categories[i]]) > len(data[categories[j]])
				}
				return categories[i] < categories[j]
			})

			for _, c := range categories {
				fmt.Printf("\nCategory: [%s] (%d entries)\n", c, len(data[c]))

				unique := make(map[string]int)
				for _, msg := range data[c] {
					unique[msg]++
				}

				uniqueMsgs := make([]string, 0, len(unique))
				for msg := range unique {
					uniqueMsgs = append(uniqueMsgs, msg)
				}
				// Most frequent first, so the dominant failure is never the one
				// truncated away.
				sort.Slice(uniqueMsgs, func(i, j int) bool {
					if unique[uniqueMsgs[i]] != unique[uniqueMsgs[j]] {
						return unique[uniqueMsgs[i]] > unique[uniqueMsgs[j]]
					}
					return uniqueMsgs[i] < uniqueMsgs[j]
				})

				shown := min(len(uniqueMsgs), maxEntriesPerCategory)
				for _, msg := range uniqueMsgs[:shown] {
					if count := unique[msg]; count > 1 {
						fmt.Printf("  - %s (x%d)\n", msg, count)
					} else {
						fmt.Printf("  - %s\n", msg)
					}
				}
				if len(uniqueMsgs) > shown {
					fmt.Printf("  ... and %d more distinct messages in this category\n", len(uniqueMsgs)-shown)
				}
			}
		}

		if len(errorsMap) == 0 && len(warnings) == 0 {
			fmt.Printf("\nNo warnings or errors found across %d log files in %s.\n", scanned, logsDir)
			return nil
		}

		printLogCategory("ERRORS", errorsMap)
		printLogCategory("WARNINGS", warnings)

		fmt.Printf("\nScanned %d log files in %s\n", scanned, logsDir)

		return nil
	},
}
