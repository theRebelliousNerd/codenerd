package typesafe

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Usage is the billable size of one request. Input tokens are the cost; output
// tokens are currently free.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response is the reply to Ask. Model is the versioned ID that answered, which
// differs from the configured alias once the alias moves; record it beside any
// threshold tuned against an answer.
type Response struct {
	Model   string
	Usage   Usage
	answers map[string]json.RawMessage
}

func (r *Response) UnmarshalJSON(b []byte) error {
	var wire struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
		Usage   *Usage                     `json:"usage"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		return err
	}
	if wire.Usage == nil || len(wire.Answers) == 0 {
		return fmt.Errorf("reply lacks answers or usage")
	}
	*r = Response{Model: wire.Model, Usage: *wire.Usage, answers: wire.Answers}
	return nil
}

// ChoiceAnswer is the winning option with the whole distribution. Confidence
// is how concentrated the distribution is, not how likely the choice is to be
// right; read Probabilities for that.
type ChoiceAnswer struct {
	Choice        string
	Confidence    float64
	Probabilities map[string]float64
}

// ScoreAnswer is the probability-weighted level, which may fall between
// integers. Probabilities and Legend share their keys (the level numbers);
// look at the distribution before trusting a middle score.
type ScoreAnswer struct {
	Score         float64
	Confidence    float64
	Legend        map[string]any
	Probabilities map[string]float64
}

// Names lists the answered question names, sorted.
func (r *Response) Names() []string {
	names := make([]string, 0, len(r.answers))
	for n := range r.answers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// answer decodes the named answer into out after checking its type tag.
func (r *Response) answer(name, want string, out any) error {
	raw, ok := r.answers[name]
	if !ok {
		return fmt.Errorf("typesafe: no answer named %q (answered: %v)", name, r.Names())
	}
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &tag); err != nil {
		return fmt.Errorf("typesafe: answer %q is malformed: %w", name, err)
	}
	if tag.Type != want {
		return fmt.Errorf("typesafe: answer %q is type %q, not %q", name, tag.Type, want)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("typesafe: answer %q is malformed: %w", name, err)
	}
	return nil
}

// Noul returns the probability, from 0 to 1, that the statement is true.
func (r *Response) Noul(name string) (float64, error) {
	var a struct {
		Noul *float64 `json:"noul"`
	}
	if err := r.answer(name, "noul", &a); err != nil {
		return 0, err
	}
	if a.Noul == nil || *a.Noul < 0 || *a.Noul > 1 {
		return 0, fmt.Errorf("typesafe: answer %q has no probability in [0,1]", name)
	}
	return *a.Noul, nil
}

// Choice returns the selected option and its distribution.
func (r *Response) Choice(name string) (ChoiceAnswer, error) {
	var a struct {
		Choice        string             `json:"choice"`
		Confidence    *float64           `json:"confidence"`
		Probabilities map[string]float64 `json:"probabilities"`
	}
	if err := r.answer(name, "choice", &a); err != nil {
		return ChoiceAnswer{}, err
	}
	if a.Choice == "" || a.Confidence == nil || len(a.Probabilities) == 0 {
		return ChoiceAnswer{}, fmt.Errorf("typesafe: answer %q lacks a choice, confidence or probabilities", name)
	}
	return ChoiceAnswer{Choice: a.Choice, Confidence: *a.Confidence, Probabilities: a.Probabilities}, nil
}

// Score returns the expected score and the level distribution.
func (r *Response) Score(name string) (ScoreAnswer, error) {
	var a struct {
		Score         *float64           `json:"score"`
		Confidence    *float64           `json:"confidence"`
		Legend        map[string]any     `json:"legend"`
		Probabilities map[string]float64 `json:"probabilities"`
	}
	if err := r.answer(name, "score", &a); err != nil {
		return ScoreAnswer{}, err
	}
	if a.Score == nil || a.Confidence == nil || len(a.Legend) == 0 || len(a.Probabilities) == 0 {
		return ScoreAnswer{}, fmt.Errorf("typesafe: answer %q lacks a score, confidence, legend or probabilities", name)
	}
	return ScoreAnswer{Score: *a.Score, Confidence: *a.Confidence, Legend: a.Legend, Probabilities: a.Probabilities}, nil
}
