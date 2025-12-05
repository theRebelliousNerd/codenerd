package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	schemas, err := os.ReadFile("internal/mangle/schemas.gl")
	if err != nil {
		panic(err)
	}
	policy, err := os.ReadFile("internal/mangle/policy.gl")
	if err != nil {
		panic(err)
	}

	var sb strings.Builder
	sb.Write(schemas)
	sb.WriteString("\n")
	sb.Write(policy)

	programStr := sb.String()
	lines := strings.Split(programStr, "\n")
	fmt.Printf("Total lines: %d\n", len(lines))

	start := len(lines) - 20
	if start < 0 {
		start = 0
	}

	fmt.Println("--- START TAIL ---")
	for i := start; i < len(lines); i++ {
		fmt.Printf("%d: %s\n", i+1, lines[i])
	}
	fmt.Println("--- END TAIL ---")
}
