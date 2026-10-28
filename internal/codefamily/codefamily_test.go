package codefamily

import (
	"strings"
	"testing"
)

func TestParse_ValidAndProjected(t *testing.T) {
	fams, err := Parse([]byte(`{"schema_version":1,"families":[{"family_id":"retry","members":["fn:a.Retry","fn:b.Retry"],"shared_requirements":["honor cancellation"]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	facts := Facts(fams)
	if len(facts) != 4 || facts[0].Predicate != "code_family" || facts[3].Predicate != "code_family_requirement" {
		t.Fatalf("facts = %v", facts)
	}
}

func TestParse_RefusesUnsoundContracts(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown field":    `{"schema_version":1,"families":[],"extra":1}`,
		"wrong version":    `{"schema_version":2,"families":[]}`,
		"one member":       `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X"],"shared_requirements":["r"]}]}`,
		"no requirement":   `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:b.X"]}]}`,
		"duplicate id":     `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:b.X"],"shared_requirements":["r"]},{"family_id":"f","members":["fn:c.X","fn:d.X"],"shared_requirements":["r"]}]}`,
		"duplicate member": `{"schema_version":1,"families":[{"family_id":"f","members":["fn:a.X","fn:a.X"],"shared_requirements":["r"]}]}`,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), FileName) {
			t.Errorf("%s: error %v does not name the file", name, err)
		}
	}
}

func TestLoad_MissingFileIsNoFamilies(t *testing.T) {
	fams, err := Load(t.TempDir())
	if err != nil || fams != nil {
		t.Fatalf("Load = %v, %v; want no families and no error", fams, err)
	}
}
