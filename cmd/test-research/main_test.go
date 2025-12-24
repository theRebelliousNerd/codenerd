package main

import (
	"strings"
	"testing"
)

func TestTruncateNoChange(t *testing.T) {
	input := "short"
	if got := truncate(input, 10); got != input {
		t.Fatalf("expected no truncation, got %q", got)
	}
}

func TestTruncateByLength(t *testing.T) {
	input := "1234567890abcdefgh"
	got := truncate(input, 10)
	if got == input {
		t.Fatalf("expected truncation")
	}
	if !strings.HasSuffix(got, "\n... (truncated)") {
		t.Fatalf("expected truncated suffix, got %q", got)
	}
}

func TestTruncateByLines(t *testing.T) {
	input := "l1\nl2\nl3\nl4\nl5\nl6\nl7\n"
	got := truncate(input, 20)
	if got == input {
		t.Fatalf("expected truncation by line count")
	}
	if !strings.HasSuffix(got, "\n... (truncated)") {
		t.Fatalf("expected truncated suffix, got %q", got)
	}
}
