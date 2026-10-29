package store

import (
	"database/sql"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"codenerd/internal/types"
)

type stringerArg struct{ v string }

func (s stringerArg) String() string { return "stringer:" + s.v }

// Every argument shape the world scanner writes, and the edges, round-trips
// through the binary codec to exactly what the JSON codec gave back.
func TestWorldArgsCodecMatchesTheJSONCodec(t *testing.T) {
	cases := [][]any{
		nil,
		{},
		{"internal/core/kernel.go", types.MangleAtom("/go"), int64(1786773933859876776), true, false},
		{nil, "", types.MangleAtom(""), int64(0), int64(-1), int64(math.MinInt64), int64(math.MaxInt64)},
		{int(7), int32(-3), uint64(42), uint64(math.MaxUint64)},
		{float32(1.5), float64(-0.25), 1e300},
		{"unicode: λ→✓", "quote \" and \\ backslash", "nul\x00byte"},
		{stringerArg{"x"}, []string{"a", "b"}},
	}
	for i, args := range cases {
		jsonText, err := encodeFactArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		want, err := decodeFactArgs(jsonText)
		if err != nil {
			t.Fatalf("case %d: JSON decode: %v", i, err)
		}
		got, err := decodeWorldArgs(encodeWorldArgs(args))
		if err != nil {
			t.Fatalf("case %d: binary decode: %v", i, err)
		}
		if len(want) == 0 && len(got) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: binary %#v, JSON %#v", i, got, want)
		}
	}
	for _, bad := range [][]byte{nil, {}, {9}, {worldArgsVersion, worldArgString, 5, 'a'}, {worldArgsVersion, 77}} {
		if _, err := decodeWorldArgs(bad); err == nil {
			t.Fatalf("decoded malformed %v", bad)
		}
	}
}

// A database whose world cache was written with JSON arguments is converted
// on open, once: every row loads back exactly as the JSON path loaded it,
// and the cache keeps working for writes and per-file reads.
func TestWorldCacheWithJSONArgumentsIsConvertedOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE world_facts (
		path TEXT NOT NULL, depth TEXT NOT NULL, fingerprint TEXT NOT NULL,
		predicate TEXT NOT NULL, args TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(path, depth, predicate, args))`); err != nil {
		t.Fatal(err)
	}
	legacy := []WorldFactInput{
		{Predicate: "file_topology", Args: []any{"a.go", "h1", types.MangleAtom("/go"), int64(1786773933859876776), types.MangleAtom("/false")}},
		{Predicate: "symbol_graph", Args: []any{"func:A", types.MangleAtom("/function"), types.MangleAtom("/public"), "a.go", "func A()"}},
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "example.com/b"}},
	}
	var want []WorldFactInput
	for _, f := range legacy {
		text, err := encodeFactArgs(f.Args)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(`INSERT INTO world_facts (path, depth, fingerprint, predicate, args) VALUES (?, 'fast', 'fp1', ?, ?)`, "a.go", f.Predicate, text); err != nil {
			t.Fatal(err)
		}
		args, _ := decodeFactArgs(text)
		want = append(want, WorldFactInput{Predicate: f.Predicate, Args: args})
	}
	if _, err := raw.Exec(`INSERT INTO world_facts (path, depth, fingerprint, predicate, args) VALUES ('bad.go', 'fast', 'fp', 'p', 'not json')`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	db, err := NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var argsType string
	if err := db.db.QueryRow(`SELECT type FROM pragma_table_info('world_facts') WHERE name = 'args'`).Scan(&argsType); err != nil || argsType != "BLOB" {
		t.Fatalf("world_facts.args type %q, %v; want BLOB", argsType, err)
	}
	got, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatal(err)
	}
	if !sameWorldFacts(got, want) {
		t.Fatalf("converted cache loads %v, want %v", got, want)
	}
	perFile, fp, err := db.LoadWorldFactsForFile("a.go", "fast")
	if err != nil || fp != "fp1" || !sameWorldFacts(perFile, want) {
		t.Fatalf("per-file load %v fp %q err %v", perFile, fp, err)
	}
	if err := db.ReplaceWorldFactsForFile("c.go", "fast", "fp2", legacy[:1]); err != nil {
		t.Fatal(err)
	}
	again, _, err := db.LoadWorldFactsForFile("c.go", "fast")
	if err != nil || !sameWorldFacts(again, want[:1]) {
		t.Fatalf("a write after conversion loads %v, %v", again, err)
	}
	_ = db.Close()
	reopened, err := NewLocalStore(path)
	if err != nil {
		t.Fatalf("reopening a converted cache: %v", err)
	}
	_ = reopened.Close()
}

func sameWorldFacts(a, b []WorldFactInput) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int)
	key := func(f WorldFactInput) string {
		return f.Predicate + "|" + reflect.ValueOf(f.Args).String() + "|" + formatArgs(f.Args)
	}
	for _, f := range a {
		seen[key(f)]++
	}
	for _, f := range b {
		if seen[key(f)] == 0 {
			return false
		}
		seen[key(f)]--
	}
	return true
}

func formatArgs(args []any) string {
	out := ""
	for _, a := range args {
		out += reflect.TypeOf(a).String() + "=" + types.Fact{Args: []any{a}}.String() + ";"
	}
	return out
}
