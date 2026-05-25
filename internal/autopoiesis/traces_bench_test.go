package autopoiesis

import (
	"testing"
)

var codeWithLog = `
func doSomething() error {
	err := someCall()
	if err != nil { log.Printf("error: %v", err) }
	return nil
}`

var codeWithoutLog = `
func doSomething() error {
	err := someCall()
	if err != nil { return err }
	return nil
}`

func BenchmarkHasErrorLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hasErrorLogging(codeWithLog)
		hasErrorLogging(codeWithoutLog)
	}
}
