// Command agent_docs regenerates the twins of every agents.md: a byte-identical
// CLAUDE.md (gitignored, for Claude Code) and the body of the directory's
// nerd.md (for codeNERD), keeping each nerd.md's own frontmatter.
//
//	go run ./cmd/tools/agent_docs          # write stale twins
//	go run ./cmd/tools/agent_docs -check   # list stale twins, exit 1 if any
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"codenerd/internal/projectdoc"
)

var osExit = os.Exit

func main() {
	root := flag.String("root", ".", "workspace root")
	check := flag.Bool("check", false, "report stale twins without writing; exit 1 if any")
	flag.Parse()
	osExit(run(*root, *check, os.Stdout, os.Stderr, projectdoc.StaleAgentDocTwins, projectdoc.WriteAgentDocTwins))
}

func run(root string, check bool, stdout, stderr io.Writer, staleFn func(string) ([]string, error), writeFn func(string) ([]string, error)) int {
	if check {
		return runCheck(root, stdout, stderr, staleFn)
	}
	return runWrite(root, stdout, stderr, writeFn)
}

func runCheck(root string, stdout, stderr io.Writer, staleFn func(string) ([]string, error)) int {
	stale, err := staleFn(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, p := range stale {
		fmt.Fprintln(stdout, p)
	}
	if len(stale) > 0 {
		fmt.Fprintf(stderr, "%d stale twin(s); run go run ./cmd/tools/agent_docs\n", len(stale))
		return 1
	}
	return 0
}

func runWrite(root string, stdout, stderr io.Writer, writeFn func(string) ([]string, error)) int {
	written, err := writeFn(root)
	for _, p := range written {
		fmt.Fprintln(stdout, "wrote", p)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%d twin(s) written\n", len(written))
	return 0
}
