package config

import (
	"encoding/json"
	"testing"
)

// `nerd config full` prints the value in force. A section absent from the
// file is in force at its WithDefaults values, not at zero.
func TestFullJSONPrintsSectionDefaultsInForce(t *testing.T) {
	out, err := FullJSON([]byte(`{"orient":{"similar_top_k":9}}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Orient  OrientConfig  `json:"orient"`
		Routing RoutingConfig `json:"routing"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	d := DefaultOrientConfig()
	if doc.Orient.SimilarTopK != 9 {
		t.Errorf("explicit orient.similar_top_k = %d, want 9", doc.Orient.SimilarTopK)
	}
	if doc.Orient.DomainAgentMax != d.DomainAgentMax || doc.Orient.SkillClusterMin != d.SkillClusterMin {
		t.Errorf("orient defaults not shown: %+v", doc.Orient)
	}
	if doc.Routing.DelegationMinConfidence != DefaultRoutingConfig().DelegationMinConfidence {
		t.Errorf("absent routing section printed %d, want its default", doc.Routing.DelegationMinConfidence)
	}
}
