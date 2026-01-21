//go:build ignore

package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

// UnusedFunction is a public function that is never called.
// This should be detected as a wiring gap.
func UnusedFunction() {
	fmt.Println("I am lonely.")
}
