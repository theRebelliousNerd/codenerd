package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"codenerd/internal/store"

	_ "modernc.org/sqlite"
)

var shardQueryDB = queryDB

func main() {
	if len(os.Args) < 2 {
		// Default: list all knowledge DBs and sample from each
		shardsDir := filepath.Join(".nerd", "shards")
		entries, err := os.ReadDir(shardsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading shards dir: %v\n", err)
			os.Exit(1)
		}

		var dbs []string
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
				continue
			}
			dbs = append(dbs, entry.Name())
		}
		sort.Strings(dbs)
		if len(dbs) == 0 {
			fmt.Fprintln(os.Stderr, "No shard databases found in .nerd/shards")
			return
		}

		// Each shard is queried concurrently into its own buffer; buffers are
		// flushed in sorted order after all workers finish so shard output is
		// deterministic and concurrent writes never touch os.Stdout.
		outs := make([][]byte, len(dbs))
		var wg sync.WaitGroup
		for i, name := range dbs {
			wg.Add(1)
			go func(idx int, shard string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "panic querying %s: %v\n", shard, r)
					}
				}()
				dbPath := filepath.Join(shardsDir, shard)
				var buf bytes.Buffer
				fmt.Fprintf(&buf, "\n=== %s ===\n", shard)
				shardQueryDB(dbPath, 5, &buf)
				outs[idx] = append([]byte(nil), buf.Bytes()...)
			}(i, name)
		}
		wg.Wait()
		for i, name := range dbs {
			if len(outs[i]) == 0 {
				continue
			}
			if _, werr := os.Stdout.Write(outs[i]); werr != nil {
				fmt.Fprintf(os.Stderr, "Error writing output for %s: %v\n", name, werr)
			}
		}
		return
	}

	dbPath := os.Args[1]
	limit := 10
	shardQueryDB(dbPath, limit, os.Stdout)
}

func queryDB(dbPath string, limit int, w io.Writer) {
	if w == nil {
		return
	}
	if dbPath == "" {
		fmt.Fprintf(w, "Error: empty database path\n")
		return
	}
	if limit <= 0 {
		limit = 10
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(w, "Error opening DB: %v\n", err)
		return
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Fprintf(w, "Error connecting to DB: %v\n", err)
		return
	}
	store.ApplyDefaultPragmas(db, store.ProfileQuery)

	// Check table schema
	tableRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		fmt.Fprintf(w, "Error querying tables: %v\n", err)
		return
	}
	defer tableRows.Close()

	var tables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			fmt.Fprintf(w, "Error scanning table row: %v\n", err)
			continue
		}
		tables = append(tables, name)
	}
	if err := tableRows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating tables: %v\n", err)
		return
	}
	fmt.Fprintf(w, "Tables: %v\n", tables)

	// Get schema of knowledge_atoms table
	schemaRows, err := db.Query("PRAGMA table_info(knowledge_atoms)")
	if err != nil {
		fmt.Fprintf(w, "No knowledge_atoms table\n")
		return
	}
	defer schemaRows.Close()
	fmt.Fprintf(w, "\nSchema:\n")
	schemaCount := 0
	for schemaRows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := schemaRows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			fmt.Fprintf(w, "  - <scan error: %v>\n", err)
			continue
		}
		schemaCount++
		fmt.Fprintf(w, "  - %s (%s)\n", name, typ)
	}
	if err := schemaRows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating schema: %v\n", err)
		return
	}
	if schemaCount == 0 {
		fmt.Fprintf(w, "No knowledge_atoms table\n")
		return
	}

	// Query all columns
	dataRows, err := db.Query("SELECT * FROM knowledge_atoms ORDER BY rowid LIMIT ?", limit)
	if err != nil {
		fmt.Fprintf(w, "Error querying knowledge: %v\n", err)
		return
	}
	defer dataRows.Close()

	cols, err := dataRows.Columns()
	if err != nil {
		fmt.Fprintf(w, "Error reading columns: %v\n", err)
		return
	}
	fmt.Fprintf(w, "\nColumns: %v\n", cols)

	fmt.Fprintf(w, "\nSample data:\n")
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────")
	i := 0
	for dataRows.Next() {
		// Scan all columns dynamically
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for k := range values {
			valuePtrs[k] = &values[k]
		}
		if err := dataRows.Scan(valuePtrs...); err != nil {
			fmt.Fprintf(w, "Scan error: %v\n", err)
			continue
		}
		i++
		fmt.Fprintf(w, "%d. ", i)
		for j, col := range cols {
			val := values[j]
			switch s := val.(type) {
			case string:
				if len(s) > 100 {
					val = s[:100] + "..."
				}
			case []byte:
				if len(s) > 100 {
					val = string(s[:100]) + "..."
				} else {
					val = string(s)
				}
			}
			fmt.Fprintf(w, "%s=%v  ", col, val)
		}
		fmt.Fprintln(w)
	}
	if err := dataRows.Err(); err != nil {
		fmt.Fprintf(w, "Error iterating rows: %v\n", err)
		return
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
	vecOK := false
	if err := db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&vecCount); err != nil {
		fmt.Fprintf(w, "Error counting vectors (table may not exist): %v\n", err)
	} else {
		fmt.Fprintf(w, "Total vectors: %d\n", vecCount)
		vecOK = true
	}

	// Sample vectors only when the vectors table was countable; this avoids
	// a duplicate error when the table is missing and surfaces schema
	// mismatches (e.g. missing id/content/metadata columns) instead of
	// silently skipping the samples.
	if vecOK {
		vecRows, err := db.Query("SELECT id, content, metadata FROM vectors ORDER BY id LIMIT ?", limit)
		if err != nil {
			fmt.Fprintf(w, "Error querying vectors: %v\n", err)
		} else {
			defer vecRows.Close()
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
			if err := vecRows.Err(); err != nil {
				fmt.Fprintf(w, "Error iterating vectors: %v\n", err)
			}
		}
	}
}
