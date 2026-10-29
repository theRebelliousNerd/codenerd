package system

import (
	"context"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// Imports past Go: a JavaScript, TypeScript, Python or Rust file the world
// model parsed names its technology through init's vocabulary, asserted at
// boot as technology_import_exact / _prefix, and a shard agent whose domain is
// that technology is needed for a task on the file. Each fixture is a source
// file parsed by the production tree-sitter walker; the roster facts are the
// boot step's own (measureShardRoster).
func TestImportsPastGoNeedTheirShardAgent(t *testing.T) {
	parser := world.NewTreeSitterParser()
	defer parser.Close()

	for _, tc := range []struct {
		name   string
		file   string
		source string
		parse  func(string, []byte) ([]world.Fact, error)
		domain string
	}{
		{"npm import", "web/app.tsx", "import React from \"react\";\nimport { useRouter } from \"next/router\";\n", parser.ParseTypeScript, "nextjs"},
		{"npm require", "server/index.js", "const express = require(\"express\");\nconst app = express();\n", parser.ParseJavaScript, "express"},
		{"python import", "app/views.py", "from django.db import models\nimport os\n", parser.ParsePython, "django"},
		{"rust use", "src/main.rs", "use tokio::sync::mpsc;\nfn main() {}\n", parser.ParseRust, "tokio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts, err := tc.parse(tc.file, []byte(tc.source))
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			roster, err := measureShardRoster(context.Background(), "", nil)
			if err != nil {
				t.Fatalf("measureShardRoster: %v", err)
			}
			k := newRosterKernel(t)
			agent := experts.AgentAtom("StackExpert")
			assert := append(append([]types.Fact{}, roster.facts...), facts...)
			assert = append(assert,
				types.Fact{Predicate: "shard_agent", Args: []any{agent}},
				types.Fact{Predicate: "shard_agent_domain", Args: []any{agent, types.MangleAtom("/" + tc.domain), types.MangleAtom("/registry")}})
			if err := k.AssertBatch(assert); err != nil {
				t.Fatalf("assert: %v", err)
			}
			sel, err := experts.Select(k, experts.Task{Verb: "/fix", Targets: []string{tc.file}})
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			if len(sel.Needs) != 1 || sel.Needs[0].Agent != "stackexpert" || sel.Needs[0].Confidence != 90 {
				t.Errorf("a fix on %s needs %+v, want stackexpert at 90 (import of %s)", tc.file, sel.Needs, tc.domain)
			}
		})
	}
}

// The separator keeps a root from matching a longer name: a file importing
// "react-dom" does not need the React expert.
func TestImportRootsRespectTheSeparator(t *testing.T) {
	parser := world.NewTreeSitterParser()
	defer parser.Close()
	facts, err := parser.ParseJavaScript("web/main.js", []byte("import { createRoot } from \"react-dom/client\";\n"))
	if err != nil {
		t.Fatal(err)
	}
	roster, err := measureShardRoster(context.Background(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	k := newRosterKernel(t)
	agent := experts.AgentAtom("ReactExpert")
	assert := append(append([]types.Fact{}, roster.facts...), facts...)
	assert = append(assert,
		types.Fact{Predicate: "shard_agent", Args: []any{agent}},
		types.Fact{Predicate: "shard_agent_domain", Args: []any{agent, types.MangleAtom("/react"), types.MangleAtom("/registry")}})
	if err := k.AssertBatch(assert); err != nil {
		t.Fatal(err)
	}
	sel, err := experts.Select(k, experts.Task{Verb: "/fix", Targets: []string{"web/main.js"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Needs) != 0 {
		t.Errorf("react-dom matched the react root: %+v", sel.Needs)
	}
}
