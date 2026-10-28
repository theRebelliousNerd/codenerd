package sessionlog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLinesJoinRestoresEveryString(t *testing.T) {
	for _, s := range []string{
		"", "one line", "a\nb", "trailing\n", "\nleading", "\n", "a\r\nb\r\n", "x\n\ny\n\n\nz",
		"unicode é世界\nnext", "<tag attr=\"v\">&amp;</tag>\n  indented",
	} {
		if got := Join(Lines(s)); got != s {
			t.Errorf("Join(Lines(%q)) = %q", s, got)
		}
	}
}

func TestSplitLinesRestoreTreeRoundTripsAWireBody(t *testing.T) {
	// Key order is deliberate and not alphabetical: it is the cache prefix.
	body := `{"model":"m","system":[{"type":"text","text":"line one\nline two\n","cache_control":{"type":"ephemeral"}}],` +
		`"messages":[{"role":"user","content":"hello\nworld"},{"role":"assistant","content":"plain"}],` +
		`"stop_sequences":["a","b\nc"],"temperature":0.2,"max_tokens":4096,"stream":false,"tools":[],"none":null}`
	tree, err := DecodeOrdered([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	split, paths := SplitLines(tree)
	wantPaths := []string{"/system/0/text", "/messages/0/content", "/stop_sequences/1"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("lines_paths = %v, want %v", paths, wantPaths)
	}

	// Through the file form and back.
	file, err := MarshalFile(split)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(file), "\n  \"model\": \"m\",\n  \"system\": [") {
		t.Fatalf("file is not 2-space indented in wire key order:\n%s", file)
	}
	back, err := DecodeOrdered(file)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreTree(back, paths)
	if err != nil {
		t.Fatal(err)
	}
	got, err := marshalPlain(restored)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("restored body differs from the wire body\n got: %s\nwant: %s", got, body)
	}
}

func TestSplitLinesLeavesGenuineArraysAlone(t *testing.T) {
	tree, _ := DecodeOrdered([]byte(`{"stop":["a","b"],"text":"x\ny"}`))
	split, paths := SplitLines(tree)
	if len(paths) != 1 || paths[0] != "/text" {
		t.Fatalf("paths = %v, want only /text", paths)
	}
	restored, err := RestoreTree(split, paths)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := marshalPlain(restored)
	if string(got) != `{"stop":["a","b"],"text":"x\ny"}` {
		t.Fatalf("restored = %s", got)
	}
}

func TestMarshalFileDoesNotEscapeHTML(t *testing.T) {
	out, err := MarshalFile(map[string]any{"p": []string{"<system>&</system>"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "<system>&</system>") {
		t.Fatalf("HTML was escaped: %s", out)
	}
	var v map[string][]string
	if err := json.Unmarshal(out, &v); err != nil || v["p"][0] != "<system>&</system>" {
		t.Fatalf("not valid JSON round trip: %v %v", err, v)
	}
}

func TestPointerEscaping(t *testing.T) {
	tree, _ := DecodeOrdered([]byte(`{"a/b":{"c~d":"x\ny"}}`))
	split, paths := SplitLines(tree)
	if len(paths) != 1 || paths[0] != "/a~1b/c~0d" {
		t.Fatalf("paths = %v", paths)
	}
	restored, err := RestoreTree(split, paths)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := marshalPlain(restored)
	if string(got) != `{"a/b":{"c~d":"x\ny"}}` {
		t.Fatalf("restored = %s", got)
	}
}
