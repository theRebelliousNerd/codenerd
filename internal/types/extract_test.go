package types

import (
	"testing"
	"time"
)

func TestExtractString(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"MangleAtom", MangleAtom("/active"), "/active"},
		{"int64", int64(42), "42"},
		{"int", 7, "7"},
		{"float64", 3.14, "3.14"},
		{"float32", float32(2.5), "2.5"},
		{"bool true", true, "/true"},
		{"bool false", false, "/false"},
		{"nil", nil, ""},
		{"time.Time", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), "2025-01-01T00:00:00Z"},
		{"time.Duration", 5 * time.Second, "5s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractString(tt.arg)
			if got != tt.want {
				t.Errorf("ExtractString(%v) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want string
	}{
		{"MangleAtom", MangleAtom("/read_file"), "/read_file"},
		{"string with /", "/coder", "/coder"},
		{"plain string", "hello", "hello"},
		{"int fallback", int64(42), "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractName(tt.arg)
			if got != tt.want {
				t.Errorf("ExtractName(%v) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestExtractInt64(t *testing.T) {
	tests := []struct {
		name   string
		arg    interface{}
		want   int64
		wantOK bool
	}{
		{"int64", int64(42), 42, true},
		{"int", 7, 7, true},
		{"float64", float64(3.9), 3, true},
		{"float32", float32(2.1), 2, true},
		{"string fails", "42", 0, false},
		{"nil fails", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractInt64(tt.arg)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ExtractInt64(%v) = (%d, %v), want (%d, %v)", tt.arg, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestExtractFloat64(t *testing.T) {
	tests := []struct {
		name   string
		arg    interface{}
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int64", int64(42), 42.0, true},
		{"int", 7, 7.0, true},
		{"string fails", "3.14", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractFloat64(tt.arg)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ExtractFloat64(%v) = (%f, %v), want (%f, %v)", tt.arg, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestExtractBool(t *testing.T) {
	tests := []struct {
		name   string
		arg    interface{}
		want   bool
		wantOK bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, false, true},
		{"atom /true", MangleAtom("/true"), true, true},
		{"atom /false", MangleAtom("/false"), false, true},
		{"string true", "true", true, true},
		{"string /true", "/true", true, true},
		{"string other", "yes", false, false},
		{"int fails", int64(1), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractBool(tt.arg)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ExtractBool(%v) = (%v, %v), want (%v, %v)", tt.arg, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestExtractTime(t *testing.T) {
	now := time.Now().UTC()
	nanos := now.UnixNano()

	got1, ok1 := ExtractTime(now)
	if !ok1 || !got1.Equal(now) {
		t.Errorf("ExtractTime(time.Time) = (%v, %v), want (%v, true)", got1, ok1, now)
	}

	got2, ok2 := ExtractTime(nanos)
	if !ok2 || got2.UnixNano() != nanos {
		t.Errorf("ExtractTime(int64) = (%v, %v), want UnixNano=%d", got2, ok2, nanos)
	}

	_, ok3 := ExtractTime("not a time")
	if ok3 {
		t.Error("ExtractTime(string) should return false")
	}
}

func TestExtractDuration(t *testing.T) {
	dur := 5 * time.Second

	got1, ok1 := ExtractDuration(dur)
	if !ok1 || got1 != dur {
		t.Errorf("ExtractDuration(Duration) = (%v, %v), want (%v, true)", got1, ok1, dur)
	}

	got2, ok2 := ExtractDuration(int64(dur))
	if !ok2 || got2 != dur {
		t.Errorf("ExtractDuration(int64) = (%v, %v), want (%v, true)", got2, ok2, dur)
	}

	_, ok3 := ExtractDuration("5s")
	if ok3 {
		t.Error("ExtractDuration(string) should return false")
	}
}

func TestArgString(t *testing.T) {
	f := Fact{Predicate: "test", Args: []interface{}{"hello", int64(42)}}

	if got := ArgString(f, 0); got != "hello" {
		t.Errorf("ArgString(f, 0) = %q, want %q", got, "hello")
	}
	if got := ArgString(f, 1); got != "42" {
		t.Errorf("ArgString(f, 1) = %q, want %q", got, "42")
	}
	// Out of bounds
	if got := ArgString(f, 5); got != "" {
		t.Errorf("ArgString(f, 5) = %q, want %q", got, "")
	}
	if got := ArgString(f, -1); got != "" {
		t.Errorf("ArgString(f, -1) = %q, want %q", got, "")
	}
}

func TestArgInt64(t *testing.T) {
	f := Fact{Predicate: "test", Args: []interface{}{int64(99), "not int"}}

	v, ok := ArgInt64(f, 0)
	if !ok || v != 99 {
		t.Errorf("ArgInt64(f, 0) = (%d, %v), want (99, true)", v, ok)
	}

	_, ok2 := ArgInt64(f, 1)
	if ok2 {
		t.Error("ArgInt64(f, 1) should return false for string arg")
	}

	_, ok3 := ArgInt64(f, 5)
	if ok3 {
		t.Error("ArgInt64(f, 5) should return false for out of bounds")
	}
}

func TestStripAtomPrefix(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/read_file", "read_file"},
		{"/coder", "coder"},
		{"no_prefix", "no_prefix"},
		{"/", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := StripAtomPrefix(tt.input); got != tt.want {
			t.Errorf("StripAtomPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
