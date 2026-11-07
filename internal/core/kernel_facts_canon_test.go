package core

import (
	"encoding/json"
	"strconv"
	"testing"
)

// A fact's key is built from one scalar definition (appendCanonScalar), so
// canonFact and canonValue agree, and every scalar keeps the form canonValue
// always gave it: name constants bare, every other string as strconv.Quote
// renders it (escaping non-printable runes and invalid UTF-8 too).
func TestCanonKeys_OneScalarDefinition(t *testing.T) {
	k := &RealKernel{}
	for _, tc := range []struct {
		v    any
		want string
	}{
		{MangleAtom("/ok"), "/ok"},
		{"plain text", strconv.Quote("plain text")},
		{"quote\" and \\slash", strconv.Quote("quote\" and \\slash")},
		{"nbsp and​zero-width", strconv.Quote("nbsp and​zero-width")},
		{"bad\xffutf8", strconv.Quote("bad\xffutf8")},
		{int64(-42), "-42"},
		{uint16(7), "7"},
		{2.5, "2.5"},
		{json.Number("12"), "12"},
		{json.Number("1.5e3"), "1500"},
		{[]byte("raw"), strconv.Quote("raw")},
		{true, "/true"},
		{nil, "null"},
	} {
		if got := canonValue(tc.v); got != tc.want {
			t.Errorf("canonValue(%#v) = %q, want %q", tc.v, got, tc.want)
		}
		key := k.canonFact(Fact{Predicate: "p", Args: []any{tc.v, int64(1)}})
		if want := "p(" + tc.want + ",1)"; key != want {
			t.Errorf("canonFact key for %#v = %q, want %q", tc.v, key, want)
		}
	}
}
