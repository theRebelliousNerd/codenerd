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
	
	db, err := sql.Open("sqlite3", "C:/CodeProjects/codeNERD/.nerd/intent_embeddings.db")
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

	// List vector tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\nTables in intent_embeddings.db:")
	count := 0
	for rows.Next() {
		var name string
		rows.Scan(&name)
		
		// Check if it's a vec0 table or normal table
		var sqlText string
		db.QueryRow("SELECT sql FROM sqlite_master WHERE name=?", name).Scan(&sqlText)
		
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
