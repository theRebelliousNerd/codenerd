package mangle

import "testing"

func TestIsNumeric(t *testing.T) {
	t.Parallel()
	numeric := []string{"0", "123", "-5", "1.5", "-0.25", "1000000"}
	for _, s := range numeric {
		if !isNumeric(s) {
			t.Errorf("isNumeric(%q)=false, want true", s)
		}
	}
	notNumeric := []string{"", "abc", "1e3", "1.2.3", "/name", "\"5\"", "12px", "-"}
	for _, s := range notNumeric {
		if isNumeric(s) {
			t.Errorf("isNumeric(%q)=true, want false", s)
		}
	}
}

func TestCompatibleTypes(t *testing.T) {
	t.Parallel()
	// ArgTypeAny accepts anything.
	for _, actual := range []ArgType{ArgTypeName, ArgTypeString, ArgTypeNumber, ArgTypeVariable, ArgTypeBool} {
		if !compatibleTypes(actual, ArgTypeAny) {
			t.Errorf("compatibleTypes(%v, Any)=false, want true", actual)
		}
	}
	// Otherwise requires exact match.
	if !compatibleTypes(ArgTypeNumber, ArgTypeNumber) {
		t.Error("identical types should be compatible")
	}
	if compatibleTypes(ArgTypeName, ArgTypeString) {
		t.Error("name vs string should be incompatible")
	}
}

func TestTypeString(t *testing.T) {
	t.Parallel()
	cases := map[ArgType]string{
		ArgTypeName:     "name constant (/...)",
		ArgTypeString:   "quoted string",
		ArgTypeNumber:   "number",
		ArgTypeVariable: "variable (Uppercase)",
		ArgTypeBool:     "boolean",
		ArgTypeAny:      "any",
	}
	for typ, want := range cases {
		if got := typeString(typ); got != want {
			t.Errorf("typeString(%v)=%q, want %q", typ, got, want)
		}
	}
}

func TestFixUnquotedStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"quotes unquoted word arg", "pred(hello world)", `pred("hello world")`},
		{"leaves name constants", "pred(/name_constant)", "pred(/name_constant)"},
		{"leaves already-quoted", `pred("already quoted")`, `pred("already quoted")`},
		{"leaves numeric", "pred(123)", "pred(123)"},
		{"leaves negative numeric", "pred(-42)", "pred(-42)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fixUnquotedStrings(tc.in); got != tc.want {
				t.Errorf("fixUnquotedStrings(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
