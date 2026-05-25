package autopoiesis

import (
	"testing"
)

var codeSnippet = `
package main
import "fmt"
import "errors"

func myTool(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty input")
	}
	if input == "error" {
		return "", errors.New("error input")
	}
	err := doSomething()
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func doSomething() error {
	return nil
}
`

func BenchmarkLogInjector_InjectErrorLogging(b *testing.B) {
	injector := NewLogInjector(LoggingRequirements{RequireErrorLog: true})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		injector.injectErrorLogging(codeSnippet, "myTool")
	}
}

func BenchmarkLogInjector_InjectAll(b *testing.B) {
	injector := NewLogInjector(LoggingRequirements{
		RequireEntryLog:     true,
		RequireExitLog:      true,
		RequireErrorLog:     true,
		RequireTimingLog:    true,
		RequireAPICallLog:   true,
		RequireIterationLog: true,
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		injector.injectEntryLogging(codeSnippet, "myTool")
		injector.injectExitLogging(codeSnippet, "myTool")
		injector.injectErrorLogging(codeSnippet, "myTool")
		injector.injectTimingLogging(codeSnippet, "myTool")
		injector.injectAPICallLogging(codeSnippet, "myTool")
		injector.injectIterationLogging(codeSnippet, "myTool")
	}
}

func BenchmarkExtractHelpers(b *testing.B) {
	text := `
1. This is a step
- This is another step
I assume that this is true.
I expect that this will happen.
Instead of doing A, I will do B.
Rather than C, D.
I chose X because Y.
Using Z since W.
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractThoughtSteps(text)
		extractAssumptions(text)
		extractAlternatives(text)
		extractDecisions(text)
	}
}
