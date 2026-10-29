// Package kbfill audits whether the knowledge databases codeNERD ships and
// builds are filled: every column of every row that should carry a value
// carries one. It is program item #14's gate ("every field of every expert KB
// and embedded database is filled, guaranteed"), in the starved-predicate
// style: Audit names each gap, the repository test
// (kbfill_test.go) holds the gaps in a baseline that may only shrink, and
// `nerd check-kb` runs the same audit over a workspace's shard-agent KBs.
//
// What "should carry" means is declared per database (specs.go): a column is
// required in every row unless its spec says when it applies or why it may be
// empty, so a column added to a schema is checked from the day it lands. A
// table is required to have rows unless its spec says why it may be empty.
package kbfill

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Column declares one column's requirement.
type Column struct {
	// When, when set, is a SQL condition: the column is required only in
	// rows matching it (a column only some kinds of row carry).
	When string
	// Optional, when set, is why the column may be empty in any row.
	Optional string
}

// Table declares one table's requirements.
type Table struct {
	// MayBeEmpty, when set, is why the table may have no rows.
	MayBeEmpty string
	// Columns names the columns that are not simply required in every row.
	Columns map[string]Column
}

// Spec declares what a kind of database must carry.
type Spec struct {
	// Name identifies the database kind in gap keys ("predicate_corpus").
	Name string
	// Tables declares the tables whose rows the database exists to hold.
	Tables map[string]Table
	// OtherTables, when set, is why a table the spec does not list may be
	// empty (a schema shared with another store). A listed-less table that
	// has rows is still audited, every column required.
	OtherTables string
}

// Gap kinds.
const (
	// GapNeverWritten: the column is empty in every row it is required in.
	GapNeverWritten = "never_written"
	// GapPartial: the column is empty in some rows it is required in.
	GapPartial = "partial"
	// GapEmptyTable: a table that must hold rows holds none.
	GapEmptyTable = "empty_table"
)

// Gap is one unfilled column or table.
type Gap struct {
	Spec   string
	Table  string
	Column string // "" for GapEmptyTable
	Kind   string
	Empty  int // rows the column is empty in
	Rows   int // rows the column is required in
}

// Key is the gap's identity in a baseline: stable while the gap stands,
// whatever the row counts.
func (g Gap) Key() string {
	if g.Column == "" {
		return fmt.Sprintf("%s %s %s", g.Spec, g.Table, g.Kind)
	}
	return fmt.Sprintf("%s %s.%s %s", g.Spec, g.Table, g.Column, g.Kind)
}

func (g Gap) String() string {
	if g.Column == "" {
		return g.Key()
	}
	return fmt.Sprintf("%s (%d/%d rows)", g.Key(), g.Empty, g.Rows)
}

// emptyValue is the SQL test for an unfilled value: NULL, an empty or
// whitespace-only string, an empty JSON array or object, or a zero-length
// blob.
func emptyValue(col string) string {
	q := quoteIdent(col)
	return fmt.Sprintf(`(%s IS NULL OR trim(CAST(%s AS TEXT)) IN ('', '[]', '{}', 'null') OR (typeof(%s) = 'blob' AND length(%s) = 0))`, q, q, q, q)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Audit reports every gap in db under spec, sorted by key.
func Audit(ctx context.Context, db *sql.DB, spec Spec) ([]Gap, error) {
	tables, err := auditedTables(ctx, db)
	if err != nil {
		return nil, err
	}
	var gaps []Gap
	for _, name := range tables {
		table, listed := spec.Tables[name]
		var rows int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdent(name)).Scan(&rows); err != nil {
			return nil, fmt.Errorf("count %s: %w", name, err)
		}
		if rows == 0 {
			switch {
			case listed && table.MayBeEmpty != "":
			case !listed && spec.OtherTables != "":
			default:
				gaps = append(gaps, Gap{Spec: spec.Name, Table: name, Kind: GapEmptyTable})
			}
			continue
		}
		cols, err := tableColumns(ctx, db, name)
		if err != nil {
			return nil, err
		}
		for _, col := range cols {
			rule := table.Columns[col]
			if rule.Optional != "" {
				continue
			}
			scope := ""
			if rule.When != "" {
				scope = " WHERE (" + rule.When + ")"
			}
			var required, empty int
			query := fmt.Sprintf("SELECT COUNT(*), COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) FROM %s%s",
				emptyValue(col), quoteIdent(name), scope)
			if err := db.QueryRowContext(ctx, query).Scan(&required, &empty); err != nil {
				return nil, fmt.Errorf("audit %s.%s: %w", name, col, err)
			}
			if empty == 0 || required == 0 {
				continue
			}
			kind := GapPartial
			if empty == required {
				kind = GapNeverWritten
			}
			gaps = append(gaps, Gap{Spec: spec.Name, Table: name, Column: col, Kind: kind, Empty: empty, Rows: required})
		}
	}
	// A listed table missing from the schema is a gap too: nothing was ever
	// written to it.
	present := map[string]bool{}
	for _, name := range tables {
		present[name] = true
	}
	for name, table := range spec.Tables {
		if !present[name] && table.MayBeEmpty == "" {
			gaps = append(gaps, Gap{Spec: spec.Name, Table: name, Kind: GapEmptyTable})
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].Key() < gaps[j].Key() })
	return gaps, nil
}

// auditedTables lists the ordinary tables of db: not SQLite's own, not a
// virtual table (the sqlite-vec indexes), not a virtual table's shadow
// tables.
func auditedTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, COALESCE(sql, '') FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var names []string
	var virtual []string
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			return nil, err
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(ddl)), "CREATE VIRTUAL TABLE") {
			virtual = append(virtual, name)
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := names[:0]
	for _, name := range names {
		shadow := false
		for _, v := range virtual {
			if strings.HasPrefix(name, v+"_") {
				shadow = true
				break
			}
		}
		if !shadow {
			out = append(out, name)
		}
	}
	return out, nil
}

func tableColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return nil, fmt.Errorf("columns of %s: %w", table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}
