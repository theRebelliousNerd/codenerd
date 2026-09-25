package gates

import "testing"

func TestCountTests_EachToolchainsConvention(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"store/store.go":           "package store\n\nfunc TestNotATest() {}\n",
		"store/store_test.go":      "package store\n\nfunc TestGet(t *testing.T) {}\nfunc TestMain(m *testing.M) {}\nfunc FuzzParse(f *testing.F) {}\nfunc helper() {}\n",
		"shop/test_cart.py":        "def test_total():\n    pass\n\nclass TestCart:\n    def test_empty(self):\n        pass\n    async def test_async(self):\n        pass\n",
		"shop/cart.py":             "def test_like_name():\n    pass\n",
		"web/app.test.ts":          "test('renders', () => {})\nit(\"works\", () => {})\ntest.each([1,2])('n %d', (n) => {})\n",
		"web/app.ts":               "test('not in a test file', () => {})\n",
		"crate/src/lib.rs":         "#[test]\nfn a() {}\n#[tokio::test]\nasync fn b() {}\n",
		"testdata/x_test.go":       "package x\n\nfunc TestFixture(t *testing.T) {}\n",
		"node_modules/m/a.test.js": "test('dep', () => {})\n",
	})
	got, err := CountTests(root)
	if err != nil {
		t.Fatal(err)
	}
	// go 2 (Get, Fuzz; not TestMain, not the non-test file), python 3,
	// ts 3, rust 2; fixtures and dependencies excluded.
	if got != 10 {
		t.Fatalf("CountTests = %d, want 10", got)
	}
}

func TestSourceLines_OneNodesOwnFiles(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"store/store.go":      "package store\n\n\nfunc Get() int {\n\treturn 1\n}\n",
		"store/store_test.go": "package store\n\nfunc TestGet(t *testing.T) {}\n",
		"store/sub/deep.go":   "package sub\n",
		"store/README.md":     "not source\n",
		"crate/src/lib.rs":    "fn a() {}\n",
		"crate/src/m/mod.rs":  "fn b() {}\n",
	})
	if got, err := SourceLines(root, []string{"store"}, false); err != nil || got != 4 {
		t.Fatalf("store lines = %d, %v; want 4 (non-blank, tests and subnodes excluded)", got, err)
	}
	if got, err := SourceLines(root, []string{"crate"}, true); err != nil || got != 2 {
		t.Fatalf("a Rust crate counts its modules: %d, %v", got, err)
	}
}

func TestCoverage_GoAndPytestCov(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want int
		ok   bool
	}{
		{"ok  \texample.com/m/store\t0.01s\tcoverage: 73.2% of statements\n", 7320, true},
		{"ok  a\tcoverage: 80.0% of statements\nok  b\tcoverage: 55.5% of statements\n", 5550, true},
		{"Name   Stmts   Miss  Cover\nTOTAL    120     30    75%\n", 7500, true},
		{"ok  \texample.com/m/store\t0.01s\n", 0, false},
	} {
		got, ok := Coverage(tc.out)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Coverage(%q) = %d, %v; want %d, %v", tc.out, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSkipDir(t *testing.T) {
	for name, want := range map[string]bool{".git": true, ".nerd": true, "node_modules": true, "testdata": true, "internal": false, "src": false} {
		if SkipDir(name) != want {
			t.Errorf("SkipDir(%q) = %v", name, !want)
		}
	}
}
