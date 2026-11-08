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
	// ts 3, rust 2, plus the testdata fixture (a member unless ignored).
	// node_modules stays out via the default ignore list.
	if got != 11 {
		t.Fatalf("CountTests = %d, want 11", got)
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

func TestCoverage_MinimumInvalidAndMixed(t *testing.T) {
	if got, ok := Coverage(""); ok || got != 0 {
		t.Fatalf("empty output = %d, %v; want 0, false", got, ok)
	}
	if got, ok := Coverage("coverage: bogus% of statements\n"); ok || got != 0 {
		t.Fatalf("unparsable percent = %d, %v; want 0, false", got, ok)
	}
	out := "ok a\tcoverage: 80.0% of statements\nok b\tcoverage: 55.5% of statements\nName   Stmts   Miss  Cover\nTOTAL    120     30    75%\n"
	if got, ok := Coverage(out); !ok || got != 5550 {
		t.Fatalf("mixed minima = %d, %v; want 5550, true", got, ok)
	}
	if got, ok := Coverage("coverage: 73.25% of statements\n"); !ok || got != 7325 {
		t.Fatalf("rounded basis points = %d, %v; want 7325, true", got, ok)
	}
}

func TestTestsIn_ExcludesTestMainAndNonTestFiles(t *testing.T) {
	if n := testsIn("store/store.go", []byte("package store\n\nfunc TestGet(t *testing.T) {}\n")); n != 0 {
		t.Fatalf("non-test go file counted %d", n)
	}
	if n := testsIn("store/store_test.go", []byte("package store\n\nfunc TestMain(m *testing.M) {}\n")); n != 0 {
		t.Fatalf("TestMain counted %d", n)
	}
	if n := testsIn("store/store_test.go", []byte("package store\n\nfunc TestA(t *testing.T) {}\nfunc FuzzB(f *testing.F) {}\n")); n != 2 {
		t.Fatalf("Test+Fuzz = %d, want 2", n)
	}
	if n := testsIn("cart.py", []byte("def test_like_name():\n    pass\n")); n != 0 {
		t.Fatalf("non-test python file counted %d", n)
	}
	if n := testsIn("crate/src/lib.rs", []byte("#[test]\nfn a() {}\n")); n != 1 {
		t.Fatalf("rust attribute = %d, want 1", n)
	}
	if !isTestFile("web/app.test.ts") || !isTestFile("web/app.spec.js") || isTestFile("web/app.ts") {
		t.Fatalf("js test-file convention wrong")
	}
	if isTestFile("crate/src/lib.rs") {
		t.Fatalf("rust file is never a test file by name")
	}
}
