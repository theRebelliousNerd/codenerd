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
	"os"

	"codenerd/internal/projectdoc"
)

func main() {
	root := flag.String("root", ".", "workspace root")
	check := flag.Bool("check", false, "report stale twins without writing; exit 1 if any")
	flag.Parse()

	if *check {
		stale, err := projectdoc.StaleAgentDocTwins(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, p := range stale {
			fmt.Println(p)
		}
		if len(stale) > 0 {
			fmt.Fprintf(os.Stderr, "%d stale twin(s); run go run ./cmd/tools/agent_docs\n", len(stale))
			os.Exit(1)
		}
		return
	}
	written, err := projectdoc.WriteAgentDocTwins(*root)
	for _, p := range written {
		fmt.Println("wrote", p)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d twin(s) written\n", len(written))
}
