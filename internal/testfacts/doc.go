// Package testfacts transduces `go test -json` streams into structured
// results and kernel facts.
//
// The gates used to scan raw `go test` output as one text blob; one run's
// blob repeated a single log line 101,144 times (46.5 MB) and blew a repair
// request to 12,066,618 tokens against a 518,288 budget (measured
// 2026-09-26). The toolchain already emits a structured event stream
// (test2json); this package parses that stream instead of the blob, so
// repeats collapse to counts while every line stays recallable.
package testfacts
