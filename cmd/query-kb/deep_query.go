//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	"codenerd/internal/store"

	_ "modernc.org/sqlite"
)

const deepQueryVectorPreviewLimit = 50

func parseDeepQueryArgs(args []string) (dbPath string, showVectors, showAtoms bool, err error) {
	if len(args) < 2 {
		return "", false, false, fmt.Errorf("Usage: go run deep_query.go <database.db> [--vectors|--atoms]")
	}
	dbPath = args[1]
	if dbPath == "" {
		return "", false, false, fmt.Errorf("empty database path: want <database.db>")
	}
	showVectors = true
	showAtoms = true
	if len(args) > 2 {
		switch args[2] {
		case "--vectors":
			showAtoms = false
		case "--atoms":
			showVectors = false
		default:
			return "", false, false, fmt.Errorf("unknown flag %q: want --vectors or --atoms", args[2])
		}
	}
	if len(args) > 3 {
		return "", false, false, fmt.Errorf("too many arguments: want <database.db> [--vectors|--atoms]")
	}
	return dbPath, showVectors, showAtoms, nil
}

func dumpDeepAtoms(db *sql.DB, stdout, stderr io.Writer) {
	if db == nil {
		if stderr != nil {
			fmt.Fprintln(stderr, "Error: nil database connection")
		}
		return
	}
	if stdout == nil || stderr == nil {
		return
	}
	if _, err := fmt.Fprintln(stdout, "\n"+strings.Repeat("=", 80)); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}
	if _, err := fmt.Fprintln(stdout, "KNOWLEDGE ATOMS (Full Content)"); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}
	if _, err := fmt.Fprintln(stdout, strings.Repeat("=", 80)); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}

	rows, err := db.Query(`SELECT id, concept, content, confidence FROM knowledge_atoms ORDER BY id`)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			fmt.Fprintf(stderr, "Error closing atoms rows: %v\n", cerr)
		}
	}()
	for rows.Next() {
		var id int
		var concept, content sql.NullString
		var confidence sql.NullFloat64
		if err := rows.Scan(&id, &concept, &content, &confidence); err != nil {
			fmt.Fprintf(stderr, "Scan error: %v\n", err)
			continue
		}
		conceptStr := ""
		if concept.Valid {
			conceptStr = concept.String
		}
		contentStr := ""
		if content.Valid {
			contentStr = content.String
		}
		confPct := 0.0
		if confidence.Valid {
			confPct = confidence.Float64 * 100
		}
		if _, err := fmt.Fprintf(stdout, "\n[%d] %s (%.0f%%)\n", id, conceptStr, confPct); err != nil {
			fmt.Fprintf(stderr, "Error writing output: %v\n", err)
			return
		}
		if _, err := fmt.Fprintf(stdout, "    %s\n", contentStr); err != nil {
			fmt.Fprintf(stderr, "Error writing output: %v\n", err)
			return
		}
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(stderr, "Error iterating atoms: %v\n", err)
	}
}

func dumpDeepVectors(db *sql.DB, stdout, stderr io.Writer) {
	if db == nil {
		if stderr != nil {
			fmt.Fprintln(stderr, "Error: nil database connection")
		}
		return
	}
	if stdout == nil || stderr == nil {
		return
	}
	if _, err := fmt.Fprintln(stdout, "\n"+strings.Repeat("=", 80)); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}
	if _, err := fmt.Fprintln(stdout, "VECTORS (Full Content with Metadata)"); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}
	if _, err := fmt.Fprintln(stdout, strings.Repeat("=", 80)); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return
	}

	rows, err := db.Query(`SELECT id, content, metadata FROM vectors ORDER BY id`)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			fmt.Fprintf(stderr, "Error closing vectors rows: %v\n", cerr)
		}
	}()
	count := 0
	for rows.Next() {
		var id int
		var content, metadata string
		if err := rows.Scan(&id, &content, &metadata); err != nil {
			fmt.Fprintf(stderr, "Scan error: %v\n", err)
			continue
		}
		count++

		// Show full content for first 50, summarize the rest once the total is known.
		if count <= deepQueryVectorPreviewLimit {
			if _, err := fmt.Fprintf(stdout, "\n[%d] %s\n", id, content); err != nil {
				fmt.Fprintf(stderr, "Error writing output: %v\n", err)
				return
			}
			if metadata != "" && metadata != "{}" {
				if _, err := fmt.Fprintf(stdout, "    META: %s\n", metadata); err != nil {
					fmt.Fprintf(stderr, "Error writing output: %v\n", err)
					return
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(stderr, "Error iterating vectors: %v\n", err)
	}
	if count > deepQueryVectorPreviewLimit {
		if _, err := fmt.Fprintf(stdout, "\n... and %d more vectors (showing first %d)\n", count-deepQueryVectorPreviewLimit, deepQueryVectorPreviewLimit); err != nil {
			fmt.Fprintf(stderr, "Error writing output: %v\n", err)
			return
		}
		if _, err := fmt.Fprintf(stdout, "Total vectors: %d\n", count); err != nil {
			fmt.Fprintf(stderr, "Error writing output: %v\n", err)
			return
		}
	}
}

func runDeepQuery(dbPath string, showVectors, showAtoms bool, stdout, stderr io.Writer) int {
	if stdout == nil || stderr == nil {
		return 1
	}
	if dbPath == "" {
		fmt.Fprintln(stderr, "empty database path: want <database.db>")
		return 1
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			fmt.Fprintf(stderr, "Error closing database: %v\n", cerr)
		}
	}()
	if err := db.Ping(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	store.ApplyDefaultPragmas(db, store.ProfileQuery)

	if showAtoms {
		dumpDeepAtoms(db, stdout, stderr)
	}
	if showVectors {
		dumpDeepVectors(db, stdout, stderr)
	}
	return 0
}

func runDeepQueryCLI(args []string, stdout, stderr io.Writer) int {
	if stdout == nil || stderr == nil {
		return 1
	}
	dbPath, showVectors, showAtoms, err := parseDeepQueryArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runDeepQuery(dbPath, showVectors, showAtoms, stdout, stderr)
}

func main() {
	os.Exit(runDeepQueryCLI(os.Args, os.Stdout, os.Stderr))
}
