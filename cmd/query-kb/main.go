package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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

		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".db" {
				dbPath := filepath.Join(shardsDir, entry.Name())
				fmt.Printf("\n=== %s ===\n", entry.Name())
				queryDB(dbPath, 5)
			}
		}
		return
	}

	dbPath := os.Args[1]
	limit := 10
	queryDB(dbPath, limit)
}

func queryDB(dbPath string, limit int) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer db.Close()

	// Check table schema
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		fmt.Printf("Error querying tables: %v\n", err)
		return
	}

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	rows.Close()
	fmt.Printf("Tables: %v\n", tables)

	// Get schema of knowledge_atoms table
	schemaRows, err := db.Query("PRAGMA table_info(knowledge_atoms)")
	if err != nil {
		fmt.Printf("No knowledge_atoms table\n")
		return
	}
	fmt.Printf("\nSchema:\n")
	for schemaRows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt interface{}
		schemaRows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk)
		fmt.Printf("  - %s (%s)\n", name, typ)
	}
	schemaRows.Close()

	// Query all columns
	rows, err = db.Query(fmt.Sprintf(`SELECT * FROM knowledge_atoms LIMIT %d`, limit))
	if err != nil {
		fmt.Printf("Error querying knowledge: %v\n", err)
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	fmt.Printf("\nColumns: %v\n", cols)

	fmt.Printf("\nSample data:\n")
	fmt.Println("─────────────────────────────────────────────────────────────")
	i := 0
	for rows.Next() {
		// Scan all columns dynamically
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		i++
		fmt.Printf("%d. ", i)
		for j, col := range cols {
			val := values[j]
			if s, ok := val.(string); ok && len(s) > 100 {
				val = s[:100] + "..."
			}
			fmt.Printf("%s=%v  ", col, val)
		}
		fmt.Println()
	}

	// Count total
	var count int
	db.QueryRow("SELECT COUNT(*) FROM knowledge_atoms").Scan(&count)
	fmt.Printf("\nTotal knowledge_atoms: %d\n", count)

	// Check vectors table
	var vecCount int
	db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&vecCount)
	fmt.Printf("Total vectors: %d\n", vecCount)

	// Sample vectors
	vecRows, err := db.Query("SELECT id, content, metadata FROM vectors LIMIT 5")
	if err == nil {
		fmt.Println("\nVector samples:")
		for vecRows.Next() {
			var id int
			var content, metadata string
			vecRows.Scan(&id, &content, &metadata)
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("  %d: %s\n", id, content)
		}
		vecRows.Close()
	}
}
