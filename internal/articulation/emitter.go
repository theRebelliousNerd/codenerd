package articulation

import (
	"encoding/json"
	"fmt"
)

// PiggybackEnvelope represents the Dual-Payload JSON Schema (v1.1.0).
// This must match perception.PiggybackEnvelope.
type PiggybackEnvelope struct {
	Surface string        `json:"surface_response"`
	Control ControlPacket `json:"control_packet"`
}

// ControlPacket contains the logic atoms and system state updates.
type ControlPacket struct {
	IntentClassification IntentClassification `json:"intent_classification"`
	MangleUpdates        []string             `json:"mangle_updates"`
	MemoryOperations     []MemoryOperation    `json:"memory_operations"`
	SelfCorrection       *SelfCorrection      `json:"self_correction,omitempty"`
}

// IntentClassification helps the kernel decide which ShardAgent to spawn.
type IntentClassification struct {
	Category   string  `json:"category"`
	Verb       string  `json:"verb"`
	Target     string  `json:"target"`
	Constraint string  `json:"constraint"`
	Confidence float64 `json:"confidence"`
}

// MemoryOperation represents a directive to the Cold Storage.
type MemoryOperation struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SelfCorrection represents an internal hypothesis about errors.
type SelfCorrection struct {
	Triggered  bool   `json:"triggered"`
	Hypothesis string `json:"hypothesis"`
}

// Emitter handles output generation.
type Emitter struct{}

func NewEmitter() *Emitter {
	return &Emitter{}
}

// Emit outputs the dual payload as JSON.
func (e *Emitter) Emit(payload PiggybackEnvelope) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
