// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run deep_query.go <database.db> [--vectors|--atoms]")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	showVectors := true
	showAtoms := true
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "--vectors":
			showAtoms = false
		case "--atoms":
			showVectors = false
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if showAtoms {
		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println("KNOWLEDGE ATOMS (Full Content)")
		fmt.Println(strings.Repeat("=", 80))

		rows, err := db.Query(`SELECT id, concept, content, confidence FROM knowledge_atoms ORDER BY id`)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			for rows.Next() {
				var id int
				var concept, content string
				var confidence float64
				rows.Scan(&id, &concept, &content, &confidence)
				fmt.Printf("\n[%d] %s (%.0f%%)\n", id, concept, confidence*100)
				fmt.Printf("    %s\n", content)
			}
			rows.Close()
		}
	}

	if showVectors {
		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println("VECTORS (Full Content with Metadata)")
		fmt.Println(strings.Repeat("=", 80))

		rows, err := db.Query(`SELECT id, content, metadata FROM vectors ORDER BY id`)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			count := 0
			for rows.Next() {
				var id int
				var content, metadata string
				rows.Scan(&id, &content, &metadata)
				count++

				// Show full content for first 50, truncated for rest
				if count <= 50 {
					fmt.Printf("\n[%d] %s\n", id, content)
					if metadata != "" && metadata != "{}" {
						fmt.Printf("    META: %s\n", metadata)
					}
				} else if count == 51 {
					fmt.Printf("\n... and %d more vectors (showing first 50)\n", count-50)
				}
			}
			rows.Close()
			if count > 50 {
				fmt.Printf("Total vectors: %d\n", count)
			}
		}
	}
}
