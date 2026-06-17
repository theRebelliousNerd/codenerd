//go:build ignore
// +build ignore

// test_vec.go is a one-off diagnostic script that verifies sqlite-vec is
// loadable and inspects the intent_embeddings.db tables. It lives at the
// repo root so it shares the cgo build env with cmd/nerd. Excluded from
// normal builds via the ignore tag; run with:
//
//	go run test_vec.go
package main

import (
	"database/sql"
	"fmt"
	"log"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	vec.Auto()

	db, err := sql.Open("sqlite3", "./.nerd/intent_embeddings.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check if sqlite-vec is loaded by querying vec_version()
	var vecVersion string
	err = db.QueryRow("SELECT vec_version()").Scan(&vecVersion)
	if err != nil {
		log.Fatalf("Failed to query vec_version: %v", err)
	}
	fmt.Printf("sqlite-vec loaded successfully! version: %s\n", vecVersion)

	// List vector tables, pulling sql text in the same query to avoid N+1 lookups
	rows, err := db.Query("SELECT name, coalesce(sql, '') FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatal(err)
	}

	var tables []string
	for rows.Next() {
		var name string
		var sqlText string
		rows.Scan(&name, &sqlText)

		tables = append(tables, name)
	}
	rows.Close() // Close rows early before iterating inner queries

	fmt.Println("\nTables in intent_embeddings.db:")
	count := 0

	for _, name := range tables {
		fmt.Printf("- %s\n", name)
		var rowCount int
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", name)).Scan(&rowCount)
		fmt.Printf("  Row count: %d\n", rowCount)
		count++
	}

	if count == 0 {
		fmt.Println("No tables found.")
	}
}
