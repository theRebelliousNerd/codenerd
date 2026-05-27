//go:build ignore

package main

import (
	"fmt"
	"github.com/google/mangle/builtin"
)

func main() {
	fmt.Println("Built-in Predicates:")
	for sym, mode := range builtin.Predicates {
		fmt.Printf("Symbol: %s, Arity: %d, Mode: %v\n", sym.Symbol, sym.Arity, mode)
	}
}
