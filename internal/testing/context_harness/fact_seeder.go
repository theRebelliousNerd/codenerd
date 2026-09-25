package context_harness

import (
	"fmt"

	"codenerd/internal/core"
)

// Scenario world state.
//
// A scenario declares the facts its world starts with as Mangle fact strings
// (Scenario.InitialFacts). The simulator parses them here and hands them to
// the context engine's SeedFacts before the first turn, so they reach both
// the kernel and the pool retrieval scores. Nothing used to: five integration
// scenarios declared campaign, issue, symbol-graph and topology facts that no
// run ever asserted, and their checkpoints validated boosts computed without
// them.
//
// The typed seeders that used to live here (SeedCampaignContext,
// SeedIssueContext, SeedSymbolGraph, ...) and FactSeeder.Clear had no caller;
// scenarios state their world as facts, and isolation between scenarios is the
// engine's Reset.

// parseInitialFacts parses a scenario's InitialFacts.
func parseInitialFacts(scenario *Scenario) ([]core.Fact, error) {
	facts := make([]core.Fact, 0, len(scenario.InitialFacts))
	for _, factStr := range scenario.InitialFacts {
		fact, err := parseMangleFact(factStr)
		if err != nil {
			return nil, fmt.Errorf("initial fact %q: %w", factStr, err)
		}
		facts = append(facts, fact)
	}
	return facts, nil
}

// parseMangleFact parses a Mangle fact string into a core.Fact.
// Example: `current_campaign("auth-migration")` -> Fact{Predicate: "current_campaign", Args: ["auth-migration"]}
func parseMangleFact(factStr string) (core.Fact, error) {
	parenIdx := -1
	for i, c := range factStr {
		if c == '(' {
			parenIdx = i
			break
		}
	}

	if parenIdx == -1 {
		if factStr == "" {
			return core.Fact{}, fmt.Errorf("empty fact")
		}
		return core.Fact{Predicate: factStr}, nil
	}
	if parenIdx == 0 {
		return core.Fact{}, fmt.Errorf("no predicate name")
	}

	predicate := factStr[:parenIdx]
	argsStr := factStr[parenIdx+1:]

	// Remove trailing period and paren.
	if len(argsStr) > 0 && argsStr[len(argsStr)-1] == '.' {
		argsStr = argsStr[:len(argsStr)-1]
	}
	if len(argsStr) > 0 && argsStr[len(argsStr)-1] == ')' {
		argsStr = argsStr[:len(argsStr)-1]
	} else {
		return core.Fact{}, fmt.Errorf("unbalanced parenthesis")
	}

	return core.Fact{
		Predicate: predicate,
		Args:      parseArgs(argsStr),
	}, nil
}

// parseArgs parses comma-separated arguments.
func parseArgs(argsStr string) []any {
	if argsStr == "" {
		return nil
	}

	var args []any
	var current []byte
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(argsStr); i++ {
		c := argsStr[i]

		if inString {
			if c == stringChar {
				inString = false
				args = append(args, string(current))
				current = nil
			} else {
				current = append(current, c)
			}
		} else if c == '"' || c == '\'' {
			inString = true
			stringChar = c
		} else if c == ',' {
			if len(current) > 0 {
				args = append(args, parseValue(string(current)))
				current = nil
			}
		} else if c != ' ' && c != '\t' {
			current = append(current, c)
		}
	}

	if len(current) > 0 {
		args = append(args, parseValue(string(current)))
	}

	return args
}

// parseValue parses an int, then a float, and otherwise returns the text
// (without an atom's leading slash).
func parseValue(s string) any {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i
	}

	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}

	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}
