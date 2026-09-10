// Tests for the Go<->SQLite serialization edge in this package: what happens
// when a value cannot be encoded on the way in, or a stored blob cannot be
// decoded on the way out.
//
// Every defect these cover had the same shape. A json.Marshal error was
// dropped, string(nil) produced "", and "" meant something different
// downstream than "no value" -- a row SQLite refuses, an embedding that
// strands its row, a DELETE that matches nothing. None of them had a symptom
// at the point of failure.

package store

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// A row whose metadata blob does not parse must still come back from a recall,
// carrying the marker that says so. The alternative -- the behaviour before
// decodeRowMetadata existed -- is that the row arrives with empty metadata and
// nothing distinguishes "this row never had metadata" from "this row lost it".
func TestDecodeRowMetadata(t *testing.T) {
	tests := []struct {
		name        string
		blob        string
		wantCorrupt bool
		wantKeys    map[string]any
	}{
		{
			name:     "no blob yields an empty map, not nil",
			blob:     "",
			wantKeys: map[string]any{},
		},
		{
			// json.Unmarshal of `null` into a map sets it to nil and reports
			// no error, and `null` is what json.Marshal writes for nil
			// metadata -- so this is the common row, not an edge case.
			name:     "the JSON null that nil metadata round-trips to",
			blob:     "null",
			wantKeys: map[string]any{},
		},
		{
			name:     "valid object decodes",
			blob:     `{"path":"a.go","k":"v"}`,
			wantKeys: map[string]any{"path": "a.go", "k": "v"},
		},
		{
			name:        "truncated object is marked",
			blob:        `{"path":"a.go"`,
			wantCorrupt: true,
		},
		{
			name:        "a JSON array is not a metadata map",
			blob:        `["path","a.go"]`,
			wantCorrupt: true,
		},
		{
			name:        "raw garbage is marked",
			blob:        `not json at all`,
			wantCorrupt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeRowMetadata(7, []byte(tt.blob))
			if got == nil {
				t.Fatal("decodeRowMetadata returned a nil map; callers write to it without a guard")
			}
			corrupt, _ := got["_corrupt_metadata"].(bool)
			if corrupt != tt.wantCorrupt {
				t.Errorf("_corrupt_metadata = %v, want %v (map: %v)", corrupt, tt.wantCorrupt, got)
			}
			for k, want := range tt.wantKeys {
				if got[k] != want {
					t.Errorf("key %q = %v, want %v", k, got[k], want)
				}
			}
			if !tt.wantCorrupt && len(got) != len(tt.wantKeys) {
				t.Errorf("decoded %v, want exactly %v", got, tt.wantKeys)
			}
		})
	}
}

// A corrupt metadata blob cannot be inserted through SQL: the partial index
// idx_vectors_predicate_content_unique evaluates json_extract on every write
// and aborts on an unparseable blob. So the row is written before the index
// exists -- which is also the only way it happens in the wild, on a database
// created by a build that predates the index, or damaged on disk.
func TestBruteForceRecallKeepsAndMarksCorruptMetadata(t *testing.T) {
	store, err := NewLocalStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(
		`INSERT INTO vectors (content, metadata, embedding) VALUES ('good', '{"k":"v"}', '[0.1,0.2,0.3,0.4]')`,
	); err != nil {
		t.Fatalf("insert good row: %v", err)
	}
	if _, err := store.db.Exec(`DROP INDEX IF EXISTS idx_vectors_predicate_content_unique`); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := store.db.Exec(
		`INSERT INTO vectors (content, metadata, embedding) VALUES ('torn', '{"k":', '[0.1,0.2,0.3,0.4]')`,
	); err != nil {
		t.Fatalf("insert torn row: %v", err)
	}

	results, err := store.vectorRecallBruteForce("q", []float32{0.1, 0.2, 0.3, 0.4}, 10)
	if err != nil {
		t.Fatalf("vectorRecallBruteForce: %v", err)
	}

	var sawGood, sawTorn bool
	for _, r := range results {
		if r.Metadata == nil {
			t.Fatalf("row %q came back with nil metadata", r.Content)
		}
		if _, ok := r.Metadata["similarity"]; !ok {
			t.Errorf("row %q is missing the similarity the recall just computed", r.Content)
		}
		corrupt, _ := r.Metadata["_corrupt_metadata"].(bool)
		switch r.Content {
		case "good":
			sawGood = true
			if corrupt {
				t.Error("a well-formed row was marked corrupt")
			}
			if r.Metadata["k"] != "v" {
				t.Errorf("good row lost its metadata: %v", r.Metadata)
			}
		case "torn":
			sawTorn = true
			if !corrupt {
				t.Errorf("torn row was not marked: %v", r.Metadata)
			}
		}
	}
	if !sawGood {
		t.Error("well-formed row missing from recall")
	}
	if !sawTorn {
		t.Error("row with unparseable metadata vanished from the recall entirely")
	}
}

// encodeRowMetadata must refuse rather than hand the empty string to SQLite,
// because the empty string is exactly the blob the vectors table rejects.
func TestEncodeRowMetadataRefusesUnserializable(t *testing.T) {
	good, err := encodeRowMetadata(map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("encodeRowMetadata on a plain map: %v", err)
	}
	if good != `{"k":"v"}` {
		t.Errorf("encoded %q, want %q", good, `{"k":"v"}`)
	}

	// A nil map is legitimate and must encode to something the index accepts.
	empty, err := encodeRowMetadata(nil)
	if err != nil {
		t.Fatalf("encodeRowMetadata(nil): %v", err)
	}
	if empty == "" {
		t.Error("nil metadata encoded to the empty string, which SQLite rejects")
	}

	for name, bad := range map[string]any{
		"NaN":     math.NaN(),
		"Inf":     math.Inf(1),
		"channel": make(chan int),
	} {
		got, err := encodeRowMetadata(map[string]any{"bad": bad})
		if err == nil {
			t.Errorf("%s: encodeRowMetadata returned %q with no error", name, got)
		}
		if got != "" {
			t.Errorf("%s: encodeRowMetadata returned %q alongside its error", name, got)
		}
	}
}

// The failure the encoder prevents, end to end: SQLite rejects the empty
// metadata blob with a message that names nothing, so a store that wrote it
// blind would report "malformed JSON" for a value problem in Go.
func TestEmptyMetadataBlobIsRejectedByTheVectorsTable(t *testing.T) {
	store, err := NewLocalStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(
		`INSERT OR REPLACE INTO vectors (content, metadata) VALUES ('c', '')`,
	); err == nil {
		t.Fatal("the vectors table accepted an empty metadata blob; " +
			"encodeRowMetadata's reason for returning an error no longer holds")
	}

	// The same row with what encodeRowMetadata produces for nil goes in fine.
	blob, err := encodeRowMetadata(nil)
	if err != nil {
		t.Fatalf("encodeRowMetadata(nil): %v", err)
	}
	if _, err := store.db.Exec(
		`INSERT OR REPLACE INTO vectors (content, metadata) VALUES ('c', ?)`, blob,
	); err != nil {
		t.Fatalf("insert with encoded nil metadata: %v", err)
	}
}

// The encoder and decoder must agree about nil, in both directions.
func TestRowMetadataRoundTrip(t *testing.T) {
	for name, in := range map[string]map[string]any{
		"nil":   nil,
		"empty": {},
		"keys":  {"path": "a.go", "kind": "predicate"},
	} {
		t.Run(name, func(t *testing.T) {
			blob, err := encodeRowMetadata(in)
			if err != nil {
				t.Fatalf("encodeRowMetadata: %v", err)
			}
			out := decodeRowMetadata(1, []byte(blob))
			if out == nil {
				t.Fatal("decoded to a nil map; callers assign into it without a guard")
			}
			out["similarity"] = 1.0 // the assignment every recall path makes
			if _, ok := out["_corrupt_metadata"]; ok {
				t.Errorf("round trip marked its own output corrupt: %v", out)
			}
			for k, want := range in {
				if out[k] != want {
					t.Errorf("key %q = %v, want %v", k, out[k], want)
				}
			}
		})
	}
}

// StoreVector must surface the encode failure instead of turning it into a
// database error about JSON.
func TestStoreVectorRejectsUnserializableMetadata(t *testing.T) {
	store, err := NewLocalStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer store.Close()

	err = store.StoreVector("c", map[string]any{"bad": math.Inf(1)})
	if err == nil {
		t.Fatal("StoreVector accepted metadata it cannot serialize")
	}
	if !strings.Contains(err.Error(), "serializable") {
		t.Errorf("error does not say what went wrong: %v", err)
	}
}

// A NaN in an embedding must not become an empty string in the database. That
// is the destructive shape: "" is not NULL, so the row stays selected by every
// recall query and then fails to parse, and the re-embed reports success.
func TestEncodeEmbeddingRefusesUnserializable(t *testing.T) {
	good, err := encodeEmbedding([]float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("encodeEmbedding on a plain vector: %v", err)
	}
	if good == "" {
		t.Fatal("a valid embedding encoded to the empty string")
	}

	for name, vec := range map[string][]float32{
		"nil":   nil,
		"empty": {},
		"NaN":   {0.1, float32(math.NaN())},
		"Inf":   {float32(math.Inf(-1)), 0.2},
	} {
		got, err := encodeEmbedding(vec)
		if err == nil {
			t.Errorf("%s: encodeEmbedding returned %q with no error", name, got)
		}
		if got != "" {
			t.Errorf("%s: encodeEmbedding returned %q alongside its error", name, got)
		}
	}
}

// The other half of that argument: an empty embedding string is NOT filtered
// out by the recall queries, so writing one really does strand the row.
func TestEmptyEmbeddingStringStillMatchesRecallQueries(t *testing.T) {
	store, err := NewLocalStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(
		`INSERT INTO vectors (content, metadata, embedding) VALUES ('stranded', '{}', '')`,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var n int
	if err := store.db.QueryRow(
		`SELECT COUNT(*) FROM vectors WHERE embedding IS NOT NULL`,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("IS NOT NULL matched %d rows, want 1: an empty embedding is no "+
			"longer selected by recall, so encodeEmbedding's reasoning has changed", n)
	}

	// It is selected, and it recalls nothing -- the row is invisible with no error.
	results, err := store.vectorRecallBruteForce("q", []float32{0.1, 0.2}, 10)
	if err != nil {
		t.Fatalf("vectorRecallBruteForce: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected the stranded row to be unrecallable, got %d results", len(results))
	}
}

// Delete must refuse args it cannot serialize rather than issue a statement
// that matches nothing. The failure it replaces is silent by construction: the
// delete succeeds, affects no rows, and returns nil, so the caller is told a
// fact was retracted while the kernel still holds it.
func TestLearningDeleteRefusesUnserializableArgs(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLearningStore: %v", err)
	}
	defer ls.Close()

	const shard, pred = "executive", "test_fact"
	if err := ls.Save(shard, pred, []any{"a", 1}, "campaign"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The real thing goes away.
	if err := ls.Delete(shard, pred, []any{"a", 1}); err != nil {
		t.Fatalf("Delete of a saved learning: %v", err)
	}

	// Args that cannot round-trip must be reported, not silently no-op'd.
	err = ls.Delete(shard, pred, []any{math.NaN()})
	if err == nil {
		t.Fatal("Delete reported success for args it cannot serialize; " +
			"the caller believes a fact was retracted that was never matched")
	}
	if !strings.Contains(err.Error(), "serializable") {
		t.Errorf("error does not say what went wrong: %v", err)
	}
}
