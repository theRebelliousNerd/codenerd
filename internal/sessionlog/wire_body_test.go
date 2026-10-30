package sessionlog

import (
	"encoding/json"
	"strings"
	"testing"
)

// An exchange file stores a wire body with its multi-line strings split into
// lines (SplitLines); nothing read one back, so RestoreTree had no caller and
// a captured call could not be reproduced. WireBody restores it exactly.
func TestWireBody_RestoresTheCapturedBody(t *testing.T) {
	original := `{"model":"m","messages":[{"role":"user","content":"line one\nline two"}],"stop":["a","b"],"temperature":0.2}`
	body, paths, note := encodeBody("application/json", []byte(original))
	textBody, _, textNote := encodeBody("text/plain", []byte("plain\nanswer"))
	ex := Exchange{Schema: ExchangeSchema, Wire: []Wire{
		{Attempt: 1, Request: WireRequest{Body: body, LinesPaths: paths, BodyNote: note}},
		{Attempt: 2, Request: WireRequest{Body: body, LinesPaths: paths, BodyNote: note}, Response: &WireResponse{Body: textBody, BodyNote: textNote}},
	}}
	data, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"line one","line two"`) {
		t.Fatalf("the fixture did not store the string as lines: %s", data)
	}

	got, err := WireBody(data, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("restored body\n%s\nwant\n%s", got, original)
	}
	if got, err := WireBody(data, 1, false); err != nil || string(got) != original {
		t.Fatalf("attempt 1 = %s (%v)", got, err)
	}
	if got, err := WireBody(data, 2, true); err != nil || string(got) != "plain\nanswer" {
		t.Fatalf("text response = %q (%v)", got, err)
	}
	if _, err := WireBody(data, 3, false); err == nil {
		t.Fatal("a missing attempt was not an error")
	}
	if _, err := WireBody(data, 1, true); err == nil {
		t.Fatal("a round trip with no response recorded was not an error")
	}
}
