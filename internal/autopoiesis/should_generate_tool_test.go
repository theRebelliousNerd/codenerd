package autopoiesis

import (
	"testing"
	"time"
)

func TestShouldGenerateToolNeed(t *testing.T) {
	baseCfg := Config{
		MinConfidence:          0.5,
		MinToolConfidence:      0.75,
		MaxToolsPerSession:     3,
		ToolGenerationCooldown: 0,
	}

	t.Run("nil need is rejected", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg}
		if o.shouldGenerateToolNeed(nil) {
			t.Error("nil need must be gated")
		}
	})

	t.Run("below MinConfidence is rejected", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg}
		need := &ToolNeed{Name: "t", Confidence: 0.4}
		if o.shouldGenerateToolNeed(need) {
			t.Error("confidence below MinConfidence must be gated")
		}
	})

	t.Run("below MinToolConfidence without strong evidence is rejected", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg}
		need := &ToolNeed{Name: "t", Confidence: 0.6, Triggers: []string{"maybe useful"}}
		if o.shouldGenerateToolNeed(need) {
			t.Error("mid confidence without strong evidence must be gated")
		}
	})

	t.Run("strong evidence overrides MinToolConfidence", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg}
		// A 'failed' trigger is strong evidence; confidence is between Min and MinTool.
		need := &ToolNeed{Name: "t", Confidence: 0.6, Triggers: []string{"previous attempt failed"}}
		if !o.shouldGenerateToolNeed(need) {
			t.Error("strong evidence (failed attempt) should pass the gate")
		}
	})

	t.Run("high confidence passes", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg}
		need := &ToolNeed{Name: "t", Confidence: 0.9}
		if !o.shouldGenerateToolNeed(need) {
			t.Error("high-confidence need should pass the gate")
		}
	})

	t.Run("session cap is enforced", func(t *testing.T) {
		o := &Orchestrator{config: baseCfg, toolsGenerated: 3}
		need := &ToolNeed{Name: "t", Confidence: 0.9}
		if o.shouldGenerateToolNeed(need) {
			t.Error("reaching MaxToolsPerSession must gate further generation")
		}
	})

	t.Run("cooldown gates without strong evidence", func(t *testing.T) {
		cfg := baseCfg
		cfg.ToolGenerationCooldown = time.Hour
		o := &Orchestrator{config: cfg, lastToolGen: time.Now()}
		need := &ToolNeed{Name: "t", Confidence: 0.9}
		if o.shouldGenerateToolNeed(need) {
			t.Error("active cooldown without strong evidence must gate generation")
		}
		// Strong evidence overrides the cooldown.
		need.Triggers = []string{"failed", "second independent trigger"}
		if !o.shouldGenerateToolNeed(need) {
			t.Error("strong evidence should override the cooldown")
		}
	})
}
