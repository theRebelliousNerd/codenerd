package main

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"fmt"
	"reflect"
)

func main() {
	t := reflect.TypeFor[ast.Clause]()
	for field := range t.Fields() {
		fmt.Printf("Field: %s\n", field.Name)
	}
}
