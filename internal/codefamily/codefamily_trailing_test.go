package codefamily

import "testing"

func TestParse_TrailingValueRejected(t *testing.T) {
	base := `{"schema_version":1,"families":[{"family_id":"fam","members":["fn:a.B","fn:c.D"],"shared_requirements":["req"]}]}`
	_, err := Parse([]byte(base + " {}"))
	if err == nil {
		t.Fatalf("Parse with trailing JSON value succeeded, want unexpected trailing data error")
	}
	if err.Error() != "code_families.json: unexpected trailing data" {
		t.Fatalf("Parse trailing value error = %q, want %q", err.Error(), "code_families.json: unexpected trailing data")
	}
}

func TestParse_TrailingSyntaxErrorWrapped(t *testing.T) {
	base := `{"schema_version":1,"families":[{"family_id":"fam","members":["fn:a.B","fn:c.D"],"shared_requirements":["req"]}]}`
	_, err := Parse([]byte(base + " {bad"))
	if err == nil {
		t.Fatalf("Parse with trailing syntax error succeeded, want error")
	}
	if err.Error() == "code_families.json: unexpected trailing data" {
		t.Fatalf("Parse trailing syntax error returned trailing-data error %q, want wrapped syntax error", err.Error())
	}
}
