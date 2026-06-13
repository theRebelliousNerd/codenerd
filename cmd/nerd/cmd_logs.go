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

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Aggregate and view system errors and warnings",
	Long:  "Scans the .nerd/logs directory for [WARN] and [ERROR] entries and groups them by category.",
	RunE: func(cmd *cobra.Command, args []string) error {
		logsDir := filepath.Join(".nerd", "logs")

		files, err := os.ReadDir(logsDir)
		if err != nil {
			return fmt.Errorf("failed to read logs directory %s: %w", logsDir, err)
		}

		// Regex to parse log lines. Example format:
		// 15:04:05.000 [api] [INFO] message
		logPattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\.\d{3}\s+\[([^\]]+)\]\s+\[(WARN|ERROR)\]\s+(.*)$`)

		warnings := make(map[string][]string)
		errorsMap := make(map[string][]string)

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

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				matches := logPattern.FindStringSubmatch(line)
				if len(matches) == 4 {
					category := matches[1]
					level := matches[2]
					msg := matches[3]

					entry := fmt.Sprintf("[%s] %s", f.Name(), msg)
					if level == "WARN" {
						warnings[category] = append(warnings[category], entry)
					} else {
						errorsMap[category] = append(errorsMap[category], entry)
					}
				}
			}
			file.Close()
		}

		printLogCategory := func(title string, data map[string][]string) {
			if len(data) == 0 {
				return
			}
			fmt.Printf("\n=== %s ===\n", title)
			categories := make([]string, 0, len(data))
			for c := range data {
				categories = append(categories, c)
			}
			sort.Strings(categories)

			for _, c := range categories {
				fmt.Printf("\nCategory: [%s] (%d entries)\n", c, len(data[c]))

				// Deduplicate
				unique := make(map[string]int)
				for _, msg := range data[c] {
					unique[msg]++
				}

				var uniqueMsgs []string
				for msg := range unique {
					uniqueMsgs = append(uniqueMsgs, msg)
				}
				sort.Strings(uniqueMsgs)

				for _, msg := range uniqueMsgs {
					count := unique[msg]
					if count > 1 {
						fmt.Printf("  - %s (x%d)\n", msg, count)
					} else {
						fmt.Printf("  - %s\n", msg)
					}
				}
			}
		}

		if len(errorsMap) == 0 && len(warnings) == 0 {
			fmt.Println("\nNo warnings or errors found in logs.")
			return nil
		}

		printLogCategory("ERRORS", errorsMap)
		printLogCategory("WARNINGS", warnings)

		return nil
	},
}
