// Package main implements the codeNERD CLI commands.
// This file contains the check-mangle command: it checks .mg files on the
// pinned mangle-go engine and, with --eval, runs them and prints what they
// derive. It is the one Mangle checker whose verdict matches the kernel's:
// the same parser, analysis and stratification the kernel runs.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"codenerd/internal/mangle"

	"github.com/spf13/cobra"
)

var (
	checkMangleStandalone bool
	checkMangleEval       []string
)

var checkMangleCmd = &cobra.Command{
	Use:   "check-mangle [file...]",
	Short: "Check .mg files on the pinned Mangle engine, or run them with --eval",
	Long: `Checks Mangle (.mg) files on the pinned mangle-go engine: parse, declarations,
arity, variable safety and stratification, exactly as the kernel loads them.

By default codeNERD's shared schemas (internal/core/defaults/schemas*.mg) load
first, so a policy file is checked in the context it runs in. --standalone checks
a program on its own, which is what an example or a scratch probe needs: a program
that declares a predicate codeNERD also declares collides with the preload.

--eval runs each file that loads and prints every fact the named predicates hold
afterwards, in the engine's own spelling (a name /a and a string "/a" stay
distinct). Use it to find out what a rule derives instead of reasoning about it.`,
	Example: `  nerd check-mangle internal/core/defaults/policy/*.mg
  nerd check-mangle --standalone --eval sibling,has_stop probe.mg`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCheckMangle,
}

func init() {
	checkMangleCmd.Flags().BoolVar(&checkMangleStandalone, "standalone", false,
		"check each file on its own, without codeNERD's shared schemas")
	checkMangleCmd.Flags().StringSliceVar(&checkMangleEval, "eval", nil,
		"evaluate each file and print the facts of these predicates (comma-separated or repeated)")
}

type checkMangleOptions struct {
	standalone bool
	eval       []string
}

func runCheckMangle(cmd *cobra.Command, args []string) error {
	opts := checkMangleOptions{standalone: checkMangleStandalone, eval: checkMangleEval}
	if !checkMangleFiles(cmd.OutOrStdout(), args, opts) {
		os.Exit(1)
	}
	return nil
}

// checkMangleFiles checks (and with opts.eval, evaluates) every file the
// patterns name, writing one verdict per file to w. It reports whether every
// file loaded and every requested predicate could be read.
func checkMangleFiles(w io.Writer, patterns []string, opts checkMangleOptions) bool {
	ok := true
	for _, pattern := range patterns {
		// Handle glob expansion (if shell didn't already)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Fprintf(w, "Error processing pattern %s: %v\n", pattern, err)
			ok = false
			continue
		}
		if len(matches) == 0 {
			// Glob returns no match (and no error) for a plain path that exists.
			if _, err := os.Stat(pattern); err == nil {
				matches = []string{pattern}
			} else {
				fmt.Fprintf(w, "No files found matching: %s\n", pattern)
				continue
			}
		}

		for _, file := range matches {
			engine, err := checkFile(w, file, opts.standalone)
			if err != nil {
				fmt.Fprintf(w, "ERROR in %s: %v\n", file, err)
				ok = false
				continue
			}
			fmt.Fprintf(w, "OK: %s\n", file)
			if len(opts.eval) > 0 && !evalMangleFile(w, engine, opts.eval) {
				ok = false
			}
		}
	}
	return ok
}

// evalMangleFile evaluates the loaded program and prints the facts of each
// named predicate. It reports whether evaluation and every read succeeded.
func evalMangleFile(w io.Writer, engine *mangle.Engine, predicates []string) bool {
	if err := engine.Evaluate(); err != nil {
		fmt.Fprintf(w, "  evaluation failed: %v\n", err)
		return false
	}
	ok := true
	for _, pred := range predicates {
		atoms, err := engine.AtomStrings(pred)
		if err != nil {
			fmt.Fprintf(w, "  %s: %v\n", pred, err)
			ok = false
			continue
		}
		fmt.Fprintf(w, "  %s: %d fact(s)\n", pred, len(atoms))
		for _, atom := range atoms {
			fmt.Fprintf(w, "    %s.\n", atom)
		}
	}
	return ok
}

// checkFile loads path into a fresh engine, after codeNERD's shared schemas
// unless standalone, and returns the engine so the caller can evaluate it.
func checkFile(w io.Writer, path string, standalone bool) (*mangle.Engine, error) {
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return nil, err
	}
	if !standalone {
		preloadSharedSchemas(w, engine, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := engine.LoadSchemaString(string(data)); err != nil {
		return nil, err
	}
	return engine, nil
}

// preloadSharedSchemas loads codeNERD's schemas*.mg (and the learning schema)
// from the first defaults directory it finds, skipping the file being checked.
// A policy file relies on those Decls, so it is checked in the context the
// kernel loads it in.
func preloadSharedSchemas(w io.Writer, engine *mangle.Engine, path string) {
	searchPaths := []string{
		"internal/core/defaults",
		".",
		"../internal/core/defaults",
		"../../internal/core/defaults",
	}
	for _, basePath := range searchPaths {
		if _, err := os.Stat(basePath); err != nil {
			continue
		}
		schemaFiles, err := filepath.Glob(filepath.Join(basePath, "schemas*.mg"))
		if err != nil {
			continue
		}
		schemaFiles = append(schemaFiles, filepath.Join(basePath, "schema", "learning.mg"))

		loaded := 0
		for _, schemaFile := range schemaFiles {
			if filepath.Base(path) == filepath.Base(schemaFile) {
				continue
			}
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				continue
			}
			if err := engine.LoadSchemaString(string(data)); err != nil {
				fmt.Fprintf(w, "WARNING: Failed to load %s: %v\n", filepath.Base(schemaFile), err)
				continue
			}
			loaded++
		}
		if loaded > 0 {
			return
		}
	}
}
