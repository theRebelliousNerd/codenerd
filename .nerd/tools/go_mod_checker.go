package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	const goModPath = "go.mod"
	
	// Check if go.mod exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		fmt.Printf("go.mod not found\n")
		os.Exit(1)
	}
	
	// Open and read go.mod
	file, err := os.Open(goModPath)
	if err != nil {
		fmt.Printf("Error opening go.mod: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()
	
	// Scan for module line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			moduleName := strings.TrimPrefix(line, "module ")
			fmt.Printf("go.mod exists\nModule name: %s\n", moduleName)
			return
		}
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading go.mod: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("go.mod exists but no module name found\n")
}