package store

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	sqlite "modernc.org/sqlite"
	"modernc.org/sqlite/vtab"
)

func init() {
	// Register sqlite-vec compat: vec0 virtual table + vector_distance_cos function.
	registerVecCompat()
}

// registerVecCompat installs the vec0 virtual table module and cosine distance
// function so sqlite-vec workflows keep working without rebuilding the driver.
func registerVecCompat() {
	_ = vtab.RegisterModule(nil, "vec0", &vecModule{})
	// Deterministic: same input blobs produce the same distance.
	_ = sqlite.RegisterDeterministicScalarFunction("vector_distance_cos", 2, vecDistanceCos)
}

// vecModule implements a minimal vec0 virtual table. It stores rows in-memory.
// Data is repopulated by the backfill step on start, so persistence across
// process restarts is not required here.
type vecModule struct {
}

// global table registry keyed by table name.
var (
	vecTablesMu sync.RWMutex
	vecTables   = make(map[string]*vecTable)
)

type vecTable struct {
	name string
	mu   sync.RWMutex
	rows []vecRow
	// next rowid to allocate (monotonic)
	nextRowID int64
}

type vecRow struct {
	rowid    int64
	embedding []byte
	content   string
	metadata  string
}

func (m *vecModule) Create(ctx vtab.Context, args []string) (vtab.Table, error) {
	return m.connect(ctx, args)
}

func (m *vecModule) Connect(ctx vtab.Context, args []string) (vtab.Table, error) {
	return m.connect(ctx, args)
}

func (m *vecModule) connect(ctx vtab.Context, args []string) (vtab.Table, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("vec0: insufficient args")
	}
	// args: [module, db, table, ...]
	name := args[2]
	if err := ctx.Declare("CREATE TABLE x(embedding BLOB, content TEXT, metadata TEXT)"); err != nil {
		return nil, err
	}

	vecTablesMu.Lock()
	defer vecTablesMu.Unlock()
	tbl, ok := vecTables[name]
	if !ok {
		tbl = &vecTable{name: name, nextRowID: 1}
		vecTables[name] = tbl
	}
	return tbl, nil
}

// BestIndex: no pushdowns; full scan.
func (t *vecTable) BestIndex(info *vtab.IndexInfo) error {
	info.EstimatedRows = int64(len(t.rows))
	return nil
}

func (t *vecTable) Open() (vtab.Cursor, error) {
	return &vecCursor{tbl: t, idx: -1}, nil
}

func (t *vecTable) Disconnect() error { return nil }
func (t *vecTable) Destroy() error    { return nil }

// Updater interface
func (t *vecTable) Insert(cols []vtab.Value, rowid *int64) error {
	if len(cols) < 3 {
		return fmt.Errorf("vec0: insert expects 3 columns")
	}
	emb, err := coerceBlob(cols[0])
	if err != nil {
		return err
	}
	content := toString(cols[1])
	meta := toString(cols[2])

	t.mu.Lock()
	defer t.mu.Unlock()
	rid := *rowid
	if rid <= 0 {
		rid = t.nextRowID
		t.nextRowID++
	}
	// Replace if existing rowid
	replaced := false
	for i := range t.rows {
		if t.rows[i].rowid == rid {
			t.rows[i] = vecRow{rowid: rid, embedding: emb, content: content, metadata: meta}
			replaced = true
			break
		}
	}
	if !replaced {
		t.rows = append(t.rows, vecRow{rowid: rid, embedding: emb, content: content, metadata: meta})
	}
	*rowid = rid
	return nil
}

func (t *vecTable) Update(oldRowid int64, cols []vtab.Value, newRowid *int64) error {
	if len(cols) < 3 {
		return fmt.Errorf("vec0: update expects 3 columns")
	}
	emb, err := coerceBlob(cols[0])
	if err != nil {
		return err
	}
	content := toString(cols[1])
	meta := toString(cols[2])

	t.mu.Lock()
	defer t.mu.Unlock()
	target := oldRowid
	if newRowid != nil && *newRowid > 0 {
		target = *newRowid
	}
	for i := range t.rows {
		if t.rows[i].rowid == oldRowid {
			t.rows[i] = vecRow{rowid: target, embedding: emb, content: content, metadata: meta}
			if target != oldRowid {
				t.rows[i].rowid = target
			}
			return nil
		}
	}
	// If not found, append.
	t.rows = append(t.rows, vecRow{rowid: target, embedding: emb, content: content, metadata: meta})
	if target >= t.nextRowID {
		t.nextRowID = target + 1
	}
	return nil
}

func (t *vecTable) Delete(oldRowid int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.rows {
		if t.rows[i].rowid == oldRowid {
			t.rows = append(t.rows[:i], t.rows[i+1:]...)
			break
		}
	}
	return nil
}

// vecCursor implements scanning.
type vecCursor struct {
	tbl *vecTable
	idx int
}

func (c *vecCursor) Filter(idxNum int, idxStr string, vals []vtab.Value) error {
	c.idx = -1
	return c.Next()
}

func (c *vecCursor) Next() error {
	c.idx++
	return nil
}

func (c *vecCursor) Eof() bool {
	c.tbl.mu.RLock()
	defer c.tbl.mu.RUnlock()
	return c.idx >= len(c.tbl.rows)
}

func (c *vecCursor) Column(col int) (vtab.Value, error) {
	c.tbl.mu.RLock()
	defer c.tbl.mu.RUnlock()
	if c.idx < 0 || c.idx >= len(c.tbl.rows) {
		return nil, fmt.Errorf("vec0: cursor out of range")
	}
	row := c.tbl.rows[c.idx]
	switch col {
	case 0:
		return row.embedding, nil
	case 1:
		return row.content, nil
	case 2:
		return row.metadata, nil
	default:
		return nil, fmt.Errorf("vec0: invalid column %d", col)
	}
}

func (c *vecCursor) Rowid() (int64, error) {
	c.tbl.mu.RLock()
	defer c.tbl.mu.RUnlock()
	if c.idx < 0 || c.idx >= len(c.tbl.rows) {
		return 0, fmt.Errorf("vec0: cursor out of range")
	}
	return c.tbl.rows[c.idx].rowid, nil
}

func (c *vecCursor) Close() error { return nil }

// vector_distance_cos implementation
func vecDistanceCos(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("vector_distance_cos expects 2 arguments")
	}
	a, err := decodeFloat32(args[0])
	if err != nil {
		return nil, err
	}
	b, err := decodeFloat32(args[1])
	if err != nil {
		return nil, err
	}
	if len(a) == 0 || len(b) == 0 {
		return float64(1), nil
	}
	if len(a) != len(b) {
		return nil, fmt.Errorf("vector_distance_cos: dimension mismatch %d vs %d", len(a), len(b))
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return float64(1), nil
	}
	cos := dot / (math.Sqrt(na) * math.Sqrt(nb))
	return float64(1 - cos), nil
}

// decodeFloat32 converts supported driver.Value types into a float32 slice.
func decodeFloat32(v driver.Value) ([]float32, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case []byte:
		if len(x)%4 != 0 {
			return nil, fmt.Errorf("vector_distance_cos: blob length %d not multiple of 4", len(x))
		}
		out := make([]float32, len(x)/4)
		for i := 0; i < len(out); i++ {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(x[i*4:]))
		}
		return out, nil
	case string:
		// treat as raw bytes
		return decodeFloat32([]byte(x))
	case []float32:
		return x, nil
	case []float64:
		out := make([]float32, len(x))
		for i, f := range x {
			out[i] = float32(f)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("vector_distance_cos: unsupported type %T", v)
	}
}

func coerceBlob(v vtab.Value) ([]byte, error) {
	switch x := v.(type) {
	case []byte:
		cp := make([]byte, len(x))
		copy(cp, x)
		return cp, nil
	case string:
		b := []byte(x)
		return b, nil
	default:
		return nil, fmt.Errorf("vec0: unsupported embedding type %T", v)
	}
}

func toString(v vtab.Value) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
