package main

import (
	"fmt"
	"reflect"
	"github.com/google/mangle/ast"
)

func main() {
	c := ast.Clause{}
	t := reflect.TypeOf(c)
	for i := 0; i < t.NumField(); i++ {
		fmt.Printf("Field: %s\n", t.Field(i).Name)
	}
}

