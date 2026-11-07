package main

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"fmt"
	"reflect"
	"strings"
)

func clauseFieldListing() string {
	return clauseFieldListingFor(reflect.TypeFor[ast.Clause]())
}

func clauseFieldListingFor(t reflect.Type) string {
	if t == nil || t.Kind() != reflect.Struct {
		return ""
	}
	var b strings.Builder
	for i := 0; i < t.NumField(); i++ {
		b.WriteString("Field: ")
		b.WriteString(t.Field(i).Name)
		b.WriteString("\n")
	}
	return b.String()
}

func main() {
	fmt.Print(clauseFieldListing())
}
