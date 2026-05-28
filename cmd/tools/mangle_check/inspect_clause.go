package main

import (
	"fmt"
	"codeberg.org/TauCeti/mangle-go/ast"
	"reflect"
)

func main() {
	t := reflect.TypeFor[ast.Clause]()
	for field := range t.Fields() {
		fmt.Printf("Field: %s\n", field.Name)
	}
}
