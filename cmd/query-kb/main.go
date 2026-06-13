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
	if len(os.Args) < 2 {
		// Default: list all knowledge DBs and sample from each
		shardsDir := filepath.Join(".nerd", "shards")
		entries, err := os.ReadDir(shardsDir)
		if err != nil {
			fmt.Printf("Error reading shards dir: %v\n", err)
			os.Exit(1)
		}

		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".db" {
				e := entry
				wg.Go(func() {
					dbPath := filepath.Join(shardsDir, e.Name())

					// We cannot intercept stdout securely concurrently using os.Stdout pipe
					// because it's a global variable and multiple goroutines writing to it
					// simultaneously will mix output.
					// We need to modify queryDB to accept an io.Writer instead.
					var buf bytes.Buffer
					fmt.Fprintf(&buf, "\n=== %s ===\n", e.Name())
					queryDB(dbPath, 5, &buf)

					mu.Lock()
					os.Stdout.Write(buf.Bytes())
					mu.Unlock()
				})
			}
		}
		wg.Wait()
		return
	}

	dbPath := os.Args[1]
	limit := 10
	queryDB(dbPath, limit, os.Stdout)
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

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			fmt.Fprintf(w, "Error scanning table row: %v\n", err)
			continue
		}
		tables = append(tables, name)
	}
	rows.Close()
	fmt.Fprintf(w, "Tables: %v\n", tables)

	// Get schema of knowledge_atoms table
	schemaRows, err := db.Query("PRAGMA table_info(knowledge_atoms)")
	if err != nil {
		fmt.Fprintf(w, "No knowledge_atoms table\n")
		return
	}
	fmt.Fprintf(w, "\nSchema:\n")
	for schemaRows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := schemaRows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			fmt.Fprintf(w, "  - <scan error: %v>\n", err)
			continue
		}
		fmt.Fprintf(w, "  - %s (%s)\n", name, typ)
	}
	schemaRows.Close()

	// Query all columns
	rows, err = db.Query(fmt.Sprintf(`SELECT * FROM knowledge_atoms LIMIT %d`, limit))
	if err != nil {
		fmt.Fprintf(w, "Error querying knowledge: %v\n", err)
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	fmt.Fprintf(w, "\nColumns: %v\n", cols)

	fmt.Fprintf(w, "\nSample data:\n")
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────")
	i := 0
	for rows.Next() {
		// Scan all columns dynamically
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
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
		for vecRows.Next() {
			var id int
			var content, metadata string
			if err := vecRows.Scan(&id, &content, &metadata); err != nil {
				fmt.Fprintf(w, "  <scan error: %v>\n", err)
				continue
			}
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Fprintf(w, "  %d: %s\n", id, content)
		}
		vecRows.Close()
	}
}
