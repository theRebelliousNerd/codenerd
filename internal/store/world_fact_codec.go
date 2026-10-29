package store

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// WORLD FACT ARGUMENT CODEC
// =============================================================================
// Boot loads the whole world cache (world_facts, ~345k rows on this
// repository) into the kernel. Its arguments used to be stored as tagged JSON
// and decoded with a json.Decoder per row: 57 MB of JSON and ~1.5-1.9 s of
// decoding per boot, on top of the scan (store.LoadAllWorldFacts 2.2-2.9 s,
// measured 2026-10-29). The world cache now stores them in a compact binary
// form decoded by a plain loop. Values round-trip exactly as the JSON codec
// round-tripped them (TestWorldArgsCodecMatchesTheJSONCodec): an int becomes
// int64, a float32 float64, a uint64 past MaxInt64 its decimal string, and an
// unknown type its %v string.
//
// Layout: one version byte, then per argument a tag byte and its payload --
// int64 zigzag varint, float64 eight big-endian bytes, bool one byte, string
// and atom a uvarint length and the bytes.

const worldArgsVersion byte = 1

const (
	worldArgNil byte = iota
	worldArgAtom
	worldArgString
	worldArgInt64
	worldArgFloat64
	worldArgBool
)

// EncodeFactArgsBinary encodes fact arguments in the binary codec. Other
// caches of parsed facts (the orientation projection) share it.
func EncodeFactArgsBinary(args []any) []byte {
	out := make([]byte, 0, 1+len(args)*16)
	out = append(out, worldArgsVersion)
	str := func(tag byte, s string) {
		out = append(out, tag)
		out = binary.AppendUvarint(out, uint64(len(s)))
		out = append(out, s...)
	}
	integer := func(v int64) {
		out = append(out, worldArgInt64)
		out = binary.AppendVarint(out, v)
	}
	float := func(v float64) {
		out = append(out, worldArgFloat64)
		out = binary.BigEndian.AppendUint64(out, math.Float64bits(v))
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case nil:
			out = append(out, worldArgNil)
		case types.MangleAtom:
			str(worldArgAtom, string(v))
		case string:
			str(worldArgString, v)
		case int:
			integer(int64(v))
		case int32:
			integer(int64(v))
		case int64:
			integer(v)
		case uint64:
			if v <= uint64(math.MaxInt64) {
				integer(int64(v))
			} else {
				str(worldArgString, strconv.FormatUint(v, 10))
			}
		case float32:
			float(float64(v))
		case float64:
			float(v)
		case bool:
			out = append(out, worldArgBool)
			if v {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
		default:
			str(worldArgString, fmt.Sprintf("%v", v))
		}
	}
	return out
}

// DecodeFactArgsBinary decodes arguments EncodeFactArgsBinary encoded.
func DecodeFactArgsBinary(data []byte) ([]any, error) {
	if len(data) == 0 || data[0] != worldArgsVersion {
		return nil, fmt.Errorf("world fact arguments: unknown encoding")
	}
	var args []any
	for i := 1; i < len(data); {
		tag := data[i]
		i++
		switch tag {
		case worldArgNil:
			args = append(args, nil)
		case worldArgAtom, worldArgString:
			n, w := binary.Uvarint(data[i:])
			if w <= 0 || uint64(len(data)-i-w) < n {
				return nil, fmt.Errorf("world fact arguments: truncated string")
			}
			i += w
			s := string(data[i : i+int(n)])
			i += int(n)
			if tag == worldArgAtom {
				args = append(args, types.MangleAtom(s))
			} else {
				args = append(args, s)
			}
		case worldArgInt64:
			v, w := binary.Varint(data[i:])
			if w <= 0 {
				return nil, fmt.Errorf("world fact arguments: truncated integer")
			}
			i += w
			args = append(args, v)
		case worldArgFloat64:
			if len(data)-i < 8 {
				return nil, fmt.Errorf("world fact arguments: truncated float")
			}
			args = append(args, math.Float64frombits(binary.BigEndian.Uint64(data[i:])))
			i += 8
		case worldArgBool:
			if i >= len(data) {
				return nil, fmt.Errorf("world fact arguments: truncated bool")
			}
			args = append(args, data[i] == 1)
			i++
		default:
			return nil, fmt.Errorf("world fact arguments: unknown tag %d", tag)
		}
	}
	return args, nil
}

// migrateWorldFactsToBinary converts a world cache written with JSON
// arguments (args TEXT) to the binary codec (args BLOB), once, in one
// transaction. Rows whose JSON did not decode were never loaded and are
// dropped. A database already on the binary schema is left alone.
func (s *LocalStore) migrateWorldFactsToBinary() error {
	var argsType string
	err := s.db.QueryRow(`SELECT type FROM pragma_table_info('world_facts') WHERE name = 'args'`).Scan(&argsType)
	if err != nil {
		return fmt.Errorf("inspect world_facts: %w", err)
	}
	if argsType != "TEXT" {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(worldFactsSchema("world_facts_binary")); err != nil {
		return fmt.Errorf("create binary world cache: %w", err)
	}
	rows, err := tx.Query(`SELECT path, depth, fingerprint, predicate, args, updated_at FROM world_facts`)
	if err != nil {
		return err
	}
	insert, err := tx.Prepare(`INSERT OR REPLACE INTO world_facts_binary (path, depth, fingerprint, predicate, args, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		rows.Close()
		return err
	}
	defer insert.Close()
	converted, dropped := 0, 0
	type row struct {
		path, depth, fingerprint, predicate, args string
		updated                                   sql.NullString
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.path, &r.depth, &r.fingerprint, &r.predicate, &r.args, &r.updated); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, r := range batch {
		args, err := decodeFactArgs(r.args)
		if err != nil {
			dropped++
			continue
		}
		if _, err := insert.Exec(r.path, r.depth, r.fingerprint, r.predicate, EncodeFactArgsBinary(args), r.updated); err != nil {
			return err
		}
		converted++
	}
	for _, stmt := range []string{
		`DROP TABLE world_facts`,
		`ALTER TABLE world_facts_binary RENAME TO world_facts`,
		worldFactsIndexes,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("swap in binary world cache: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	logging.Store("World cache converted to binary arguments: %d rows (%d undecodable dropped)", converted, dropped)
	return nil
}

// worldFactsSchema is the world cache table under a name.
func worldFactsSchema(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
		path TEXT NOT NULL,
		depth TEXT NOT NULL, -- fast | deep
		fingerprint TEXT NOT NULL,
		predicate TEXT NOT NULL,
		args BLOB NOT NULL, -- world_fact_codec.go
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(path, depth, predicate, args)
	);`
}

const worldFactsIndexes = `
	CREATE INDEX IF NOT EXISTS idx_world_facts_predicate ON world_facts(predicate);
	CREATE INDEX IF NOT EXISTS idx_world_facts_depth ON world_facts(depth);
	CREATE INDEX IF NOT EXISTS idx_world_facts_path ON world_facts(path);
`
