package perception

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Wire keys of the classification answer. The envelope nests the
// understanding under understandingKey; the bare compatibility profile is the
// understanding object itself.
const understandingKey = "understanding"

// requiredClassificationFields are the keys the routing decision cannot do
// without: the action becomes the verb, and the confidence gates clarification
// and delegation. An answer that omits one has not classified the turn:
// decoding it anyway yields an empty action that maps to /explain and a zero
// confidence indistinguishable from an uncertain reading.
//
// primary_intent, semantic_type and domain are deliberately not required:
// primary_intent is a one-word summary nothing reads, semantic_type only
// refines a category the action already fixes (it adds "instruction" and a
// question signal), and an unset domain maps like "general". The vocabulary of
// the values is the kernel's (understanding_vocab_miss, perception_routing.mg),
// not a Go allowlist; this list asks only whether the field is present.
var requiredClassificationFields = []string{"action_type", "confidence"}

// maxConfidencePercent bounds the documented percentage form of confidence.
const maxConfidencePercent = 100

// parseResponse admits a model's answer as a classification, or reports the
// structural fault that makes it not one.
//
// The shape is chosen from the top-level wire keys before either variant is
// decoded. An object with an "understanding" key is an envelope, and an
// envelope that cannot be decoded is an error: it is never re-read as a bare
// understanding, because the bare decode ignores the nested object and returns
// an all-zero understanding with a nil error -- an empty interpretation that
// looks like a success. The bare form (the understanding object itself) stays
// supported as the documented compatibility profile.
func (t *LLMTransducer) parseResponse(response string) (*Understanding, error) {
	jsonStr := ExtractCleanJSON(response)
	if jsonStr == "" {
		return nil, &ClassificationError{Stage: StageOutput, Code: "no_json"}
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &top); err != nil {
		return nil, &ClassificationError{Stage: StageDecode, Code: "not_an_object", Cause: err}
	}

	var (
		understanding Understanding
		fields        map[string]json.RawMessage
	)
	if raw, isEnvelope := top[understandingKey]; isEnvelope {
		if !isJSONObject(raw) {
			return nil, &ClassificationError{Stage: StageDecode, Code: "envelope_understanding_not_object"}
		}
		var envelope UnderstandingEnvelope
		if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
			return nil, &ClassificationError{Stage: StageDecode, Code: "envelope_" + decodeFaultCode(err), Cause: err}
		}
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, &ClassificationError{Stage: StageDecode, Code: "envelope_" + decodeFaultCode(err), Cause: err}
		}
		understanding = envelope.Understanding
		understanding.SurfaceResponse = envelope.SurfaceResponse
		if envelope.Brief != nil {
			understanding.Brief = envelope.Brief
		}
	} else {
		if err := json.Unmarshal([]byte(jsonStr), &understanding); err != nil {
			return nil, &ClassificationError{Stage: StageDecode, Code: "bare_" + decodeFaultCode(err), Cause: err}
		}
		fields = top
	}

	if err := admitUnderstanding(&understanding, fields); err != nil {
		return nil, err
	}
	normalizeLLMFields(&understanding)
	return &understanding, nil
}

// admitUnderstanding checks what the wire shape alone cannot: that the fields
// the routing decision reads were supplied (presence, not value, so an
// explicit confidence of 0 stays distinct from an omitted one), and that the
// confidence is a number the gates can compare.
func admitUnderstanding(u *Understanding, fields map[string]json.RawMessage) error {
	var missing []string
	for _, key := range requiredClassificationFields {
		raw, present := fields[key]
		if !present || strings.TrimSpace(string(raw)) == "null" {
			missing = append(missing, key)
			continue
		}
		if key == "action_type" && strings.TrimSpace(u.ActionType) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return &ClassificationError{Stage: StageRequiredFields, Code: "missing:" + strings.Join(missing, ",")}
	}
	// Confidence is a ratio, or the documented percentage form (above 1 up to
	// 100, converted by normalizeLLMFields). Anything else is not a confidence:
	// accepting it clamped would read a malformed number as a stronger or
	// weaker opinion than the model gave.
	if math.IsNaN(u.Confidence) || math.IsInf(u.Confidence, 0) || u.Confidence < 0 || u.Confidence > maxConfidencePercent {
		return &ClassificationError{Stage: StageRequiredFields, Code: "confidence_out_of_range"}
	}
	return nil
}

// decodeFaultCode reduces a decoder error to a safe code: the field and the
// wire type that failed, never the value.
func decodeFaultCode(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("field_type:%s", typeErr.Field)
	}
	return "decode"
}
