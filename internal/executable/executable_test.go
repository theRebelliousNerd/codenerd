package executable

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestKind(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		body     []byte
		want     bool
		wantKind string
	}{
		"elf":                   {[]byte{0x7F, 'E', 'L', 'F', 2, 1}, true, "ELF (Linux)"},
		"pe":                    {[]byte("MZ\x90\x00"), true, "PE (Windows)"},
		"macho64":               {[]byte{0xCF, 0xFA, 0xED, 0xFE, 7}, true, "Mach-O 64 (macOS)"},
		"macho32":               {[]byte{0xCE, 0xFA, 0xED, 0xFE, 7}, true, "Mach-O 32 (macOS)"},
		"macho-universal":       {[]byte{0xCA, 0xFE, 0xBA, 0xBE, 0}, true, "Mach-O universal (macOS)"},
		"script.sh":             {[]byte("#!/bin/sh\necho hi\n"), false, ""},
		"source.go":             {[]byte("package main\n"), false, ""},
		"empty":                 {nil, false, ""},
		"short":                 {[]byte{0x7F}, false, ""},
		"truncated-mz":          {[]byte{'M'}, false, ""},
		"truncated-elf":         {[]byte{0x7F, 'E'}, false, ""},
		"truncated-macho64-1":   {[]byte{0xCF}, false, ""},
		"truncated-macho64-3":   {[]byte{0xCF, 0xFA, 0xED}, false, ""},
		"truncated-universal-2": {[]byte{0xCA, 0xFE}, false, ""},
		"near-miss-macho":       {[]byte{0xCF, 0xFA, 0xED, 0xFF}, false, ""},
		"pe-lower":              {[]byte{'m', 'z'}, false, ""},
		"zeroed-2":              {[]byte{0x00, 0x00}, false, ""},
	}
	for name, c := range cases {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, c.body, 0o755); err != nil {
			t.Fatal(err)
		}
		gotKind, got := Kind(p)
		if got != c.want {
			t.Errorf("%s: Kind = %v, want %v", name, got, c.want)
		}
		if c.want && gotKind != c.wantKind {
			t.Errorf("%s: Kind name = %q, want %q", name, gotKind, c.wantKind)
		}
		if !c.want && gotKind != "" {
			t.Errorf("%s: Kind name = %q, want empty", name, gotKind)
		}
	}
	if _, ok := Kind(filepath.Join(dir, "missing")); ok {
		t.Error("a missing file was reported as an executable")
	}
	if _, ok := Kind(""); ok {
		t.Error("an empty path was reported as an executable")
	}
	if _, ok := Kind(dir); ok {
		t.Error("a directory was reported as an executable")
	}
	kindOfCases := map[string]struct {
		header []byte
		want   bool
	}{
		"nil":               {nil, false},
		"empty":             {[]byte{}, false},
		"single-0x7f":       {[]byte{0x7F}, false},
		"single-M":          {[]byte{'M'}, false},
		"partial-elf":       {[]byte{0x7F, 'E', 'L'}, false},
		"partial-macho64":   {[]byte{0xCF, 0xFA, 0xED}, false},
		"partial-universal": {[]byte{0xCA, 0xFE}, false},
		"near-miss-macho":   {[]byte{0xCF, 0xFA, 0xED, 0xFF}, false},
		"pe-lower":          {[]byte{'m', 'z'}, false},
		"elf":               {[]byte{0x7F, 'E', 'L', 'F'}, true},
		"elf-trailing":      {[]byte{0x7F, 'E', 'L', 'F', 2, 1, 0}, true},
		"mz":                {[]byte{'M', 'Z'}, true},
		"macho32":           {[]byte{0xCE, 0xFA, 0xED, 0xFE}, true},
		"macho64":           {[]byte{0xCF, 0xFA, 0xED, 0xFE}, true},
		"macho-universal":   {[]byte{0xCA, 0xFE, 0xBA, 0xBE}, true},
		"text":              {[]byte("#!/bin/sh\n"), false},
	}
	for name, c := range kindOfCases {
		if _, got := KindOf(c.header); got != c.want {
			t.Errorf("KindOf %s = %v, want %v", name, got, c.want)
		}
	}
}

func TestKind_ErrorAndEdgeBranches(t *testing.T) {
	k, ok := Kind(filepath.Join(t.TempDir(), "does-not-exist"))
	if ok || k != "" {
		t.Fatalf("Kind(missing) = %q,%v want %q,%v", k, ok, "", false)
	}
	dir := t.TempDir()
	if k, ok := Kind(dir); ok || k != "" {
		t.Fatalf("Kind(dir) = %q,%v want %q,%v", k, ok, "", false)
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if k, ok := Kind(empty); ok || k != "" {
		t.Fatalf("Kind(empty) = %q,%v want %q,%v", k, ok, "", false)
	}
	short := filepath.Join(t.TempDir(), "short")
	if err := os.WriteFile(short, []byte{0x7f}, 0o644); err != nil {
		t.Fatal(err)
	}
	if k, ok := Kind(short); ok || k != "" {
		t.Fatalf("Kind(short) = %q,%v want %q,%v", k, ok, "", false)
	}
	two := filepath.Join(t.TempDir(), "two")
	if err := os.WriteFile(two, []byte{0x7f, 0x45}, 0o644); err != nil {
		t.Fatal(err)
	}
	if k, ok := Kind(two); ok || k != "" {
		t.Fatalf("Kind(two) = %q,%v want %q,%v", k, ok, "", false)
	}
	if k, ok := KindOf(nil); ok || k != "" {
		t.Fatalf("KindOf(nil) = %q,%v want %q,%v", k, ok, "", false)
	}
	if k, ok := KindOf([]byte{}); ok || k != "" {
		t.Fatalf("KindOf(empty) = %q,%v want %q,%v", k, ok, "", false)
	}
	if k, ok := KindOf([]byte{0x7f}); ok || k != "" {
		t.Fatalf("KindOf(short) = %q,%v want %q,%v", k, ok, "", false)
	}
}

func TestKind_ReadErrorIsNotExecutable(t *testing.T) {
	if _, err := os.Stat("/proc/self/mem"); err != nil {
		t.Skip("no /proc/self/mem on this platform")
	}
	k, ok := Kind("/proc/self/mem")
	if ok || k != "" {
		t.Fatalf("Kind(/proc/self/mem) = %q,%v want %q,%v", k, ok, "", false)
	}
}

func TestKindOf_PinExactNames(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   string
		wantOK bool
	}{
		{"elf", []byte{0x7F, 'E', 'L', 'F'}, "ELF (Linux)", true},
		{"elfTrailing", []byte{0x7F, 'E', 'L', 'F', 2, 1, 0}, "ELF (Linux)", true},
		{"pe", []byte{'M', 'Z'}, "PE (Windows)", true},
		{"peTrailing", []byte("MZ\x90\x00"), "PE (Windows)", true},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)", true},
		{"macho64Trailing", []byte{0xCF, 0xFA, 0xED, 0xFE, 7}, "Mach-O 64 (macOS)", true},
		{"macho32", []byte{0xCE, 0xFA, 0xED, 0xFE}, "Mach-O 32 (macOS)", true},
		{"machoUniversal", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "Mach-O universal (macOS)", true},
		{"nil", nil, "", false},
		{"empty", []byte{}, "", false},
		{"single7F", []byte{0x7F}, "", false},
		{"singleM", []byte{'M'}, "", false},
		{"partialElf", []byte{0x7F, 'E', 'L'}, "", false},
		{"truncatedElf2", []byte{0x7F, 'E'}, "", false},
		{"text", []byte("#!/bin/sh\n"), "", false},
		{"source", []byte("package main\n"), "", false},
	}
	for _, tc := range cases {
		got, ok := KindOf(tc.header)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("KindOf(%s) = %q,%v want %q,%v", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestKind_PinFileKinds(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name  string
		body  []byte
		want  string
		wantO bool
	}{
		{"elfExact", []byte{0x7F, 'E', 'L', 'F'}, "ELF (Linux)", true},
		{"elfTrailing", []byte{0x7F, 'E', 'L', 'F', 2, 1}, "ELF (Linux)", true},
		{"peExact", []byte{'M', 'Z'}, "PE (Windows)", true},
		{"peTrailing", []byte("MZ\x90\x00"), "PE (Windows)", true},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)", true},
		{"macho32", []byte{0xCE, 0xFA, 0xED, 0xFE}, "Mach-O 32 (macOS)", true},
		{"machoUniversal", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "Mach-O universal (macOS)", true},
		{"script", []byte("#!/bin/sh\necho hi\n"), "", false},
		{"empty", nil, "", false},
		{"short1", []byte{0x7F}, "", false},
		{"shortMZ", []byte{'M'}, "", false},
		{"shortElf2", []byte{0x7F, 'E'}, "", false},
		{"shortElf3", []byte{0x7F, 'E', 'L'}, "", false},
	}
	for _, tc := range cases {
		p := filepath.Join(dir, tc.name)
		if err := os.WriteFile(p, tc.body, 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := Kind(p)
		if ok != tc.wantO || got != tc.want {
			t.Errorf("Kind(%s) = %q,%v want %q,%v", tc.name, got, ok, tc.want, tc.wantO)
		}
	}
	if got, ok := Kind(""); ok || got != "" {
		t.Errorf("Kind(empty path) = %q,%v want %q,%v", got, ok, "", false)
	}
	if got, ok := Kind(filepath.Join(dir, "does-not-exist")); ok || got != "" {
		t.Errorf("Kind(missing) = %q,%v want %q,%v", got, ok, "", false)
	}
	if got, ok := Kind(dir); ok || got != "" {
		t.Errorf("Kind(dir) = %q,%v want %q,%v", got, ok, "", false)
	}
}

func TestKindOf_PinPrefixSemantics(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   string
		wantOK bool
	}{
		{"elfExact", []byte{0x7F, 'E', 'L', 'F'}, "ELF (Linux)", true},
		{"elfTrailing", []byte{0x7F, 'E', 'L', 'F', 2, 1, 0, 9}, "ELF (Linux)", true},
		{"elfOffsetByOne", []byte{0x00, 0x7F, 'E', 'L', 'F'}, "", false},
		{"elfLeadingJunk", []byte{'#', '!', 0x7F, 'E', 'L', 'F'}, "", false},
		{"peExact", []byte{'M', 'Z'}, "PE (Windows)", true},
		{"peTrailing", []byte{'M', 'Z', 0x90, 0x00, 0x03}, "PE (Windows)", true},
		{"peLower", []byte{'m', 'z'}, "", false},
		{"peOffset", []byte{0x00, 'M', 'Z'}, "", false},
		{"macho64Exact", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)", true},
		{"macho32Exact", []byte{0xCE, 0xFA, 0xED, 0xFE}, "Mach-O 32 (macOS)", true},
		{"machoUniversal", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "Mach-O universal (macOS)", true},
		{"machoUniversalTrailing", []byte{0xCA, 0xFE, 0xBA, 0xBE, 0x00, 0x01}, "Mach-O universal (macOS)", true},
		{"zeroed4", []byte{0x00, 0x00, 0x00, 0x00}, "", false},
		{"text", []byte("#!/bin/sh\n"), "", false},
	}
	for _, tc := range cases {
		got, ok := KindOf(tc.header)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("KindOf(%s) = %q,%v want %q,%v", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
	if got, _ := KindOf([]byte{0xCE, 0xFA, 0xED, 0xFE}); got == "Mach-O 64 (macOS)" {
		t.Errorf("KindOf(macho32) confused with 64: %q", got)
	}
	if got, _ := KindOf([]byte{0xCF, 0xFA, 0xED, 0xFE}); got == "Mach-O 32 (macOS)" {
		t.Errorf("KindOf(macho64) confused with 32: %q", got)
	}
}

func TestKind_PinSymlinkAndAgreement(t *testing.T) {
	dir := t.TempDir()
	elfPath := filepath.Join(dir, "a-elf")
	if err := os.WriteFile(elfPath, []byte{0x7F, 'E', 'L', 'F', 2, 1}, 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link-elf")
	if err := os.Symlink(elfPath, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if got, ok := Kind(linkPath); !ok || got != "ELF (Linux)" {
		t.Errorf("Kind(symlink to ELF) = %q,%v want %q,%v", got, ok, "ELF (Linux)", true)
	}
	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "no-such-target"), dangling); err == nil {
		if got, ok := Kind(dangling); ok || got != "" {
			t.Errorf("Kind(dangling symlink) = %q,%v want %q,%v", got, ok, "", false)
		}
	}
	probes := []struct {
		name string
		body []byte
	}{
		{"elf", []byte{0x7F, 'E', 'L', 'F'}},
		{"pe", []byte{'M', 'Z'}},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE}},
		{"macho32", []byte{0xCE, 0xFA, 0xED, 0xFE}},
		{"universal", []byte{0xCA, 0xFE, 0xBA, 0xBE}},
	}
	for _, p := range probes {
		fp := filepath.Join(dir, "agree-"+p.name)
		full := append(append([]byte{}, p.body...), 0x00, 0x01, 0x02)
		if err := os.WriteFile(fp, full, 0o644); err != nil {
			t.Fatal(err)
		}
		wantName, wantOK := KindOf(p.body)
		gotName, gotOK := Kind(fp)
		if gotOK != wantOK || gotName != wantName {
			t.Errorf("Kind(%s) = %q,%v disagrees with KindOf = %q,%v", p.name, gotName, gotOK, wantName, wantOK)
		}
	}
	offPath := filepath.Join(dir, "offset-elf")
	if err := os.WriteFile(offPath, []byte{0x00, 0x7F, 'E', 'L', 'F'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Kind(offPath); ok || got != "" {
		t.Errorf("Kind(offset ELF) = %q,%v want %q,%v", got, ok, "", false)
	}
}

func TestKind_StabilizeTruncatedAndReaderFailures(t *testing.T) {
	if got := maxHeaderLen(); got != 4 {
		t.Fatalf("maxHeaderLen() = %d, want 4", got)
	}
	if got, ok := kindOfReader(failKindReader{}); ok || got != "" {
		t.Fatalf("kindOfReader(failing) = %q,%v want %q,%v", got, ok, "", false)
	}
	if got, ok := kindOfReader(emptyKindReader{}); ok || got != "" {
		t.Fatalf("kindOfReader(empty) = %q,%v want %q,%v", got, ok, "", false)
	}
	truncated := map[string][]byte{
		"macho64-1":       {0xCF},
		"macho64-2":       {0xCF, 0xFA},
		"macho64-3":       {0xCF, 0xFA, 0xED},
		"macho32-1":       {0xCE},
		"macho32-2":       {0xCE, 0xFA},
		"macho32-3":       {0xCE, 0xFA, 0xED},
		"universal-1":     {0xCA},
		"universal-2":     {0xCA, 0xFE},
		"universal-3":     {0xCA, 0xFE, 0xBA},
		"pe-lower":        {'m', 'z'},
		"elf-offset":      {0x00, 0x7F, 'E', 'L', 'F'},
		"zeroed-short":    {0x00},
		"zeroed-2":        {0x00, 0x00},
		"near-miss-macho": {0xCF, 0xFA, 0xED, 0xFF},
	}
	for name, header := range truncated {
		if got, ok := KindOf(header); ok || got != "" {
			t.Errorf("KindOf(%s) = %q,%v want %q,%v", name, got, ok, "", false)
		}
	}
	dir := t.TempDir()
	for name, header := range truncated {
		p := filepath.Join(dir, "trunc-"+name)
		if err := os.WriteFile(p, header, 0o644); err != nil {
			t.Fatal(err)
		}
		if got, ok := Kind(p); ok || got != "" {
			t.Errorf("Kind(%s) = %q,%v want %q,%v", name, got, ok, "", false)
		}
	}
}
func TestKindOfReader_EmptyMagicTable(t *testing.T) {
	orig := magic
	magic = nil
	defer func() { magic = orig }()
	if got := maxHeaderLen(); got != 0 {
		t.Fatalf("maxHeaderLen() with empty table = %d, want 0", got)
	}
	if got, ok := kindOfReader(emptyKindReader{}); ok || got != "" {
		t.Fatalf("kindOfReader with empty magic table = %q,%v want %q,%v", got, ok, "", false)
	}
}

type kindTestErr string

func (e kindTestErr) Error() string { return string(e) }

type failKindReader struct{}

func (failKindReader) Read([]byte) (int, error) { return 0, kindTestErr("boom") }

type emptyKindReader struct{}

func (emptyKindReader) Read([]byte) (int, error) { return 0, io.EOF }

func TestKind_PinKindStabilizedGuards(t *testing.T) {
	dir := t.TempDir()
	if got, ok := Kind(""); ok || got != "" {
		t.Fatalf("Kind(empty path) = %q,%v want %q,%v", got, ok, "", false)
	}
	if got, ok := Kind(filepath.Join(dir, "does-not-exist")); ok || got != "" {
		t.Fatalf("Kind(missing) = %q,%v want %q,%v", got, ok, "", false)
	}
	if got, ok := Kind(dir); ok || got != "" {
		t.Fatalf("Kind(dir) = %q,%v want %q,%v", got, ok, "", false)
	}
	emptyPath := filepath.Join(dir, "empty-file")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Kind(emptyPath); ok || got != "" {
		t.Fatalf("Kind(empty file) = %q,%v want %q,%v", got, ok, "", false)
	}
	shortPath := filepath.Join(dir, "short-file")
	if err := os.WriteFile(shortPath, []byte{0x7F}, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Kind(shortPath); ok || got != "" {
		t.Fatalf("Kind(short file) = %q,%v want %q,%v", got, ok, "", false)
	}
	positives := []struct {
		name string
		body []byte
		want string
	}{
		{"elf", []byte{0x7F, 'E', 'L', 'F', 2, 1}, "ELF (Linux)"},
		{"peExact2", []byte{'M', 'Z'}, "PE (Windows)"},
		{"pe", []byte{'M', 'Z', 0x90, 0x00}, "PE (Windows)"},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE}, "Mach-O 64 (macOS)"},
		{"macho32", []byte{0xCE, 0xFA, 0xED, 0xFE}, "Mach-O 32 (macOS)"},
		{"universal", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "Mach-O universal (macOS)"},
	}
	for _, tc := range positives {
		p := filepath.Join(dir, "pos-"+tc.name)
		if err := os.WriteFile(p, tc.body, 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := Kind(p)
		if !ok || got != tc.want {
			t.Errorf("Kind(%s) = %q,%v want %q,%v", tc.name, got, ok, tc.want, true)
		}
	}
}

func TestKindOfReader_PinHealthyPaths(t *testing.T) {
	healthy := []struct {
		name string
		body []byte
		want string
	}{
		{"elf", []byte{0x7F, 'E', 'L', 'F', 2, 1}, "ELF (Linux)"},
		{"pe", []byte{'M', 'Z', 0x90}, "PE (Windows)"},
		{"macho64", []byte{0xCF, 0xFA, 0xED, 0xFE, 9}, "Mach-O 64 (macOS)"},
	}
	for _, tc := range healthy {
		got, ok := kindOfReader(bytes.NewReader(tc.body))
		if !ok || got != tc.want {
			t.Errorf("kindOfReader(%s) = %q,%v want %q,%v", tc.name, got, ok, tc.want, true)
		}
	}
	if got, ok := kindOfReader(bytes.NewReader(nil)); ok || got != "" {
		t.Errorf("kindOfReader(empty) = %q,%v want %q,%v", got, ok, "", false)
	}
}

func TestMaxHeaderLen_PinLongestRegardlessOfOrder(t *testing.T) {
	if got := maxHeaderLen(); got != 4 {
		t.Fatalf("maxHeaderLen() = %d, want 4", got)
	}
	old := magic
	defer func() { magic = old }()
	magic = []struct {
		name  string
		magic []byte
	}{
		{"two", []byte{'M', 'Z'}},
		{"five", []byte{1, 2, 3, 4, 5}},
		{"three", []byte{1, 2, 3}},
	}
	if got := maxHeaderLen(); got != 5 {
		t.Fatalf("maxHeaderLen(reordered) = %d, want 5", got)
	}
}

func TestKindOfReader_NilReaderIsNotExecutable(t *testing.T) {
	if got, ok := kindOfReader(nil); got != "" || ok {
		t.Fatalf("kindOfReader(nil) = (%q, %v), want (\"\", false)", got, ok)
	}
}
