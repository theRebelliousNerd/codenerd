package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Look for go.mod file in current directory and parents
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found go.mod, read and parse it
			content, err := ioutil.ReadFile(goModPath)
			if err != nil {
				log.Fatal(err)
			}

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "module ") {
					moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module "))
					fmt.Printf("Go module name: %s\n", moduleName)
					return
				}
			}
			fmt.Println("go.mod found but no module declaration")
			return
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			fmt.Println("No go.mod file found")
			return
		}
		dir = parent
	}
}