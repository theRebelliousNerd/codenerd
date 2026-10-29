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
	for b.Loop() {
		hasErrorLogging(codeWithLog)
		hasErrorLogging(codeWithoutLog)
	}
}
