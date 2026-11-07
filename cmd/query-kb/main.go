package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"codenerd/internal/store"

	_ "modernc.org/sqlite"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

const usage = "Usage: query-kb [database.db [--deep|--vectors|--atoms]]\n" +
	"  no argument: sample every .nerd/shards/*.db\n" +
	"  database.db: schema and sample rows\n" +
	"  --deep: full atom and vector content; --vectors or --atoms: one of them\n"

// run is the command: it writes to w and returns the exit code.
func run(args []string, w io.Writer) int {
	switch len(args) {
	case 0:
		return sampleShardDBs(filepath.Join(".nerd", "shards"), w)
	case 1:
		queryDB(args[0], 10, w)
		return 0
	case 2:
		switch args[1] {
		case "--deep":
			runDeepQuery(args[0], true, true, w)
		case "--vectors":
			runDeepQuery(args[0], true, false, w)
		case "--atoms":
			runDeepQuery(args[0], false, true, w)
		default:
			fmt.Fprintf(w, "unknown option %q\n%s", args[1], usage)
			return 2
		}
		return 0
	default:
		fmt.Fprint(w, usage)
		return 2
	}
}

// sampleShardDBs samples every .db in dir concurrently; each database's
// report is buffered and written whole, so reports never interleave.
func sampleShardDBs(dir string, w io.Writer) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(w, "Error reading shards dir: %v\n", err)
		return 1
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".db" {
			continue
		}
		name := entry.Name()
		wg.Go(func() {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "\n=== %s ===\n", name)
			queryDB(filepath.Join(dir, name), 5, &buf)
			mu.Lock()
			w.Write(buf.Bytes())
			mu.Unlock()
		})
	}
	wg.Wait()
	return 0
}

// queryRowIter captures the subset of *sql.Rows used by the query-kb print
// loops so deterministic unit tests can inject scan and iteration failures
// without corrupting a database file. *sql.Rows satisfies it in production.
type queryRowIter interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// collectTableNames drains table-name rows, reporting per-row scan failures
// and a terminal iteration failure without aborting the listing.
func collectTableNames(rows queryRowIter, w io.Writer) []string {
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			fmt.Fprintf(w, "Error scanning table row: %v\n", err)
			continue
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating tables: %v\n", err)
	}
	return tables
}

// printSchemaRows drains PRAGMA table_info rows, reporting per-row scan
// failures and a terminal iteration failure.
func printSchemaRows(rows queryRowIter, w io.Writer) {
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			fmt.Fprintf(w, "  - <scan error: %v>\n", err)
			continue
		}
		fmt.Fprintf(w, "  - %s (%s)\n", name, typ)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "  - <iteration error: %v>\n", err)
	}
}

// printKnowledgeSamples drains SELECT * FROM knowledge_atoms rows using the
// supplied column names, reporting per-row scan failures and a terminal
// iteration failure.
func printKnowledgeSamples(rows queryRowIter, cols []string, w io.Writer) {
	i := 0
	for rows.Next() {
		// Scan all columns dynamically
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for k := range values {
			valuePtrs[k] = &values[k]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			fmt.Fprintf(w, "Scan error: %v\n", err)
			continue
		}
		i++
		fmt.Fprintf(w, "%d. ", i)
		for j, col := range cols {
			val := values[j]
			if s, ok := val.(string); ok && len(s) > 100 {
				val = s[:100] + "..."
			}
			fmt.Fprintf(w, "%s=%v  ", col, val)
		}
		fmt.Fprintln(w)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating knowledge: %v\n", err)
	}
}

// printVectorSamples drains vector sample rows, reporting per-row scan
// failures and a terminal iteration failure.
func printVectorSamples(rows queryRowIter, w io.Writer) {
	for rows.Next() {
		var id int
		var content, metadata string
		if err := rows.Scan(&id, &content, &metadata); err != nil {
			fmt.Fprintf(w, "  <scan error: %v>\n", err)
			continue
		}
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Fprintf(w, "  %d: %s\n", id, content)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(w, "  <iteration error: %v>\n", err)
	}
}

func queryDB(dbPath string, limit int, w io.Writer) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(w, "Error opening DB: %v\n", err)
		return
	}
	defer db.Close()
	store.ApplyDefaultPragmas(db, store.ProfileQuery)

	// Check table schema
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		fmt.Fprintf(w, "Error querying tables: %v\n", err)
		return
	}

	tables := collectTableNames(rows, w)
	rows.Close()
	fmt.Fprintf(w, "Tables: %v\n", tables)

	// Get schema of knowledge_atoms table
	schemaRows, err := db.Query("PRAGMA table_info(knowledge_atoms)")
	if err != nil {
		fmt.Fprintf(w, "No knowledge_atoms table\n")
		return
	}
	fmt.Fprintf(w, "\nSchema:\n")
	printSchemaRows(schemaRows, w)
	schemaRows.Close()

	// Query all columns
	rows, err = db.Query("SELECT * FROM knowledge_atoms LIMIT ?", limit)
	if err != nil {
		fmt.Fprintf(w, "Error querying knowledge: %v\n", err)
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	fmt.Fprintf(w, "\nColumns: %v\n", cols)

	fmt.Fprintf(w, "\nSample data:\n")
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────")
	printKnowledgeSamples(rows, cols, w)

	// Count total
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM knowledge_atoms").Scan(&count); err != nil {
		fmt.Fprintf(w, "\nError counting knowledge_atoms: %v\n", err)
	} else {
		fmt.Fprintf(w, "\nTotal knowledge_atoms: %d\n", count)
	}

	// Check vectors table
	var vecCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&vecCount); err != nil {
		fmt.Fprintf(w, "Error counting vectors (table may not exist): %v\n", err)
	} else {
		fmt.Fprintf(w, "Total vectors: %d\n", vecCount)
	}

	// Sample vectors
	vecRows, err := db.Query("SELECT id, content, metadata FROM vectors LIMIT 5")
	if err == nil {
		fmt.Fprintln(w, "\nVector samples:")
		printVectorSamples(vecRows, w)
		vecRows.Close()
	}
}
