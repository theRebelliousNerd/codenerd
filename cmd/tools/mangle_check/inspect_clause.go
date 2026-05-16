package main

import (
	"fmt"
	"github.com/google/mangle/ast"
	"reflect"
)

func main() {
	c := ast.Clause{}
	t := reflect.TypeOf(c)
	for i := 0; i < t.NumField(); i++ {
		fmt.Printf("Field: %s\n", t.Field(i).Name)
	}
}
