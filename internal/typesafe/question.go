package typesafe

import (
	"encoding/json"
	"fmt"
)

// Per-question limits from the API spec; checked here so a bad question fails
// before it costs a round trip.
const (
	maxChoiceOptions = 255
	maxScoreLevels   = 10
)

// Question is one named question in a request. Instructions and criteria
// accept a string, an object or an array, as the API does.
type Question interface {
	json.Marshaler
	validate() error
}

// NoulCriteria says what a yes and a no mean. Either side may be nil.
type NoulCriteria struct {
	True  any `json:"true,omitempty"`
	False any `json:"false,omitempty"`
}

// Noul asks for the probability that a statement is true.
type Noul struct {
	Instructions any
	Criteria     *NoulCriteria
}

// Choice picks one of the named options. Criteria maps an option name to its
// description; a nil description leaves the option to its name alone.
type Choice struct {
	Instructions any
	Criteria     map[string]any
}

// Score rates against ordered levels; a level's position is its score,
// starting at zero.
type Score struct {
	Instructions any
	Criteria     []any
}

func (q Noul) validate() error {
	if q.Instructions == nil {
		return fmt.Errorf("noul question needs instructions")
	}
	return nil
}

func (q Choice) validate() error {
	if q.Instructions == nil {
		return fmt.Errorf("choice question needs instructions")
	}
	if n := len(q.Criteria); n < 1 || n > maxChoiceOptions {
		return fmt.Errorf("choice question needs 1 to %d options, has %d", maxChoiceOptions, n)
	}
	return nil
}

func (q Score) validate() error {
	if q.Instructions == nil {
		return fmt.Errorf("score question needs instructions")
	}
	if n := len(q.Criteria); n < 1 || n > maxScoreLevels {
		return fmt.Errorf("score question needs 1 to %d levels, has %d", maxScoreLevels, n)
	}
	for i, level := range q.Criteria {
		if level == nil {
			return fmt.Errorf("score level %d is null", i)
		}
	}
	return nil
}

// The wire shapes put "type" first and leave out what is unset.
type noulWire struct {
	Type         string        `json:"type"`
	Instructions any           `json:"instructions"`
	Criteria     *NoulCriteria `json:"criteria,omitempty"`
}

type choiceWire struct {
	Type         string         `json:"type"`
	Instructions any            `json:"instructions"`
	Criteria     map[string]any `json:"criteria"`
}

type scoreWire struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     []any  `json:"criteria"`
}

func (q Noul) MarshalJSON() ([]byte, error) {
	c := q.Criteria
	if c != nil && c.True == nil && c.False == nil {
		c = nil
	}
	return json.Marshal(noulWire{"noul", q.Instructions, c})
}

func (q Choice) MarshalJSON() ([]byte, error) {
	return json.Marshal(choiceWire{"choice", q.Instructions, q.Criteria})
}

func (q Score) MarshalJSON() ([]byte, error) {
	return json.Marshal(scoreWire{"score", q.Instructions, q.Criteria})
}
