package main

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"codenerd/internal/store"

	_ "modernc.org/sqlite"
)

// printDeepAtoms drains full knowledge-atom rows, reporting per-row scan
// failures and a terminal iteration failure. It takes queryRowIter so
// deterministic tests can inject failures without corrupting a database
// file; *sql.Rows satisfies it in production.
func printDeepAtoms(rows queryRowIter, w io.Writer) {
	for rows.Next() {
		var id int
		var concept, content string
		var confidence float64
		if err := rows.Scan(&id, &concept, &content, &confidence); err != nil {
			fmt.Fprintf(w, "Error scanning atom row: %v\n", err)
			continue
		}
		fmt.Fprintf(w, "\n[%d] %s (%.0f%%)\n", id, concept, confidence*100)
		fmt.Fprintf(w, "    %s\n", content)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating atoms: %v\n", err)
	}
}

// printDeepVectors drains full vector rows, reporting per-row scan failures
// and a terminal iteration failure. It returns the number of successfully
// scanned rows so callers can report totals. It takes queryRowIter so
// deterministic tests can inject failures without corrupting a database
// file; *sql.Rows satisfies it in production.
func printDeepVectors(rows queryRowIter, w io.Writer) int {
	count := 0
	for rows.Next() {
		var id int
		var content, metadata string
		if err := rows.Scan(&id, &content, &metadata); err != nil {
			fmt.Fprintf(w, "Error scanning vector row: %v\n", err)
			continue
		}
		count++

		// Show full content for first 50, truncated for rest
		if count <= 50 {
			fmt.Fprintf(w, "\n[%d] %s\n", id, content)
			if metadata != "" && metadata != "{}" {
				fmt.Fprintf(w, "    META: %s\n", metadata)
			}
		} else if count == 51 {
			fmt.Fprintf(w, "\n... and %d more vectors (showing first 50)\n", count-50)
		}
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating vectors: %v\n", err)
	}
	return count
}

// runDeepQuery prints full atom and vector content for dbPath to w.
// showVectors and showAtoms select which sections run. All row iteration
// goes through printDeepAtoms and printDeepVectors so every Scan and Err
// branch stays reachable by deterministic queryRowIter fakes.
func runDeepQuery(dbPath string, showVectors, showAtoms bool, w io.Writer) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(w, "Error: %v\n", err)
		return
	}
	defer db.Close()
	store.ApplyDefaultPragmas(db, store.ProfileQuery)

	if showAtoms {
		fmt.Fprintln(w, "\n"+strings.Repeat("=", 80))
		fmt.Fprintln(w, "KNOWLEDGE ATOMS (Full Content)")
		fmt.Fprintln(w, strings.Repeat("=", 80))

		rows, err := db.Query(`SELECT id, concept, content, confidence FROM knowledge_atoms ORDER BY id`)
		if err != nil {
			fmt.Fprintf(w, "Error: %v\n", err)
		} else {
			printDeepAtoms(rows, w)
			rows.Close()
		}
	}

	if showVectors {
		fmt.Fprintln(w, "\n"+strings.Repeat("=", 80))
		fmt.Fprintln(w, "VECTORS (Full Content with Metadata)")
		fmt.Fprintln(w, strings.Repeat("=", 80))

		rows, err := db.Query(`SELECT id, content, metadata FROM vectors ORDER BY id`)
		if err != nil {
			fmt.Fprintf(w, "Error: %v\n", err)
		} else {
			count := printDeepVectors(rows, w)
			rows.Close()
			if count > 50 {
				fmt.Fprintf(w, "Total vectors: %d\n", count)
			}
		}
	}
}
