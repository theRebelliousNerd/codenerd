package main

import (
	"fmt"
	"github.com/google/mangle/ast"
	"reflect"
)

func main() {
	t := reflect.TypeFor[ast.Clause]()
	for field := range t.Fields() {
		fmt.Printf("Field: %s\n", field.Name)
	}
}
