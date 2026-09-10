package broker

// Pass-through accessors.
//
// These are unconditional on purpose, and the reasoning is worth recording
// because it is the difference between a safe decorator and a subtly broken one.
//
// A probe like `if p, ok := client.(interface{ GetLastThinkingTokens() int });
// ok` has two possible outcomes at the call site: skip, or use the value. Every
// accessor below is an "enrich if available" read whose caller leaves its field
// at the zero value when the probe fails. Forwarding the call and returning the
// underlying's zero value when it does not implement the method therefore
// produces exactly the behaviour the un-wrapped client produced.
//
// The same argument holds for the setters: a probe that succeeds and then calls
// a method which forwards to nothing is indistinguishable from a probe that
// failed and skipped the call.
//
// It does NOT hold for the three methods in optional.go. Those select a control
// flow — the native tool loop instead of a synthesized envelope, structured
// output instead of free text, thought streaming instead of plain streaming —
// and answering "yes, supported" for a client that cannot serve them routes the
// request into a path that fails. Those stay conditional, which is what wrap.go
// is for.

// GetModel returns the model this broker is metering.
//
// Unlike the accessors below, this one answers from the broker's own state
// rather than only forwarding, because the broker tracks the model for counting
// and is authoritative after a SetModel that the underlying client ignored.
func (c *core) GetModel() string {
	if getter, ok := c.underlying.(interface{ GetModel() string }); ok {
		if model := getter.GetModel(); model != "" {
			return model
		}
	}
	return c.currentModel()
}

// DisableSemaphore forwards a concurrency-limit opt-out.
func (c *core) DisableSemaphore() {
	if d, ok := c.underlying.(interface{ DisableSemaphore() }); ok {
		d.DisableSemaphore()
	}
}

// GetLastThinkingTokens forwards the last turn's thinking token count.
func (c *core) GetLastThinkingTokens() int {
	if p, ok := c.underlying.(interface{ GetLastThinkingTokens() int }); ok {
		return p.GetLastThinkingTokens()
	}
	return 0
}

// GetThinkingLevel forwards the configured thinking level.
func (c *core) GetThinkingLevel() string {
	if p, ok := c.underlying.(interface{ GetThinkingLevel() string }); ok {
		return p.GetThinkingLevel()
	}
	return ""
}

// GetLastThoughtSummary forwards the last turn's reasoning summary.
func (c *core) GetLastThoughtSummary() string {
	if p, ok := c.underlying.(interface{ GetLastThoughtSummary() string }); ok {
		return p.GetLastThoughtSummary()
	}
	return ""
}

// GetLastThoughtSignature forwards the last turn's opaque reasoning signature.
//
// This value is provider-bound continuation state, not text. It is forwarded
// verbatim and never inspected, rewritten, or synthesized here.
func (c *core) GetLastThoughtSignature() string {
	if p, ok := c.underlying.(interface{ GetLastThoughtSignature() string }); ok {
		return p.GetLastThoughtSignature()
	}
	return ""
}

// GetLastGroundingSources implements types.GroundingProvider.
func (c *core) GetLastGroundingSources() []string {
	if p, ok := c.underlying.(interface{ GetLastGroundingSources() []string }); ok {
		return p.GetLastGroundingSources()
	}
	return nil
}

// IsGoogleSearchEnabled implements types.GroundingProvider.
func (c *core) IsGoogleSearchEnabled() bool {
	if p, ok := c.underlying.(interface{ IsGoogleSearchEnabled() bool }); ok {
		return p.IsGoogleSearchEnabled()
	}
	return false
}

// IsURLContextEnabled implements types.GroundingProvider.
func (c *core) IsURLContextEnabled() bool {
	if p, ok := c.underlying.(interface{ IsURLContextEnabled() bool }); ok {
		return p.IsURLContextEnabled()
	}
	return false
}

// SetEnableGoogleSearch implements types.GroundingController.
func (c *core) SetEnableGoogleSearch(enable bool) {
	if p, ok := c.underlying.(interface{ SetEnableGoogleSearch(bool) }); ok {
		p.SetEnableGoogleSearch(enable)
	}
}

// SetEnableURLContext implements types.GroundingController.
func (c *core) SetEnableURLContext(enable bool) {
	if p, ok := c.underlying.(interface{ SetEnableURLContext(bool) }); ok {
		p.SetEnableURLContext(enable)
	}
}

// SetURLContextURLs implements types.GroundingController.
func (c *core) SetURLContextURLs(urls []string) {
	if p, ok := c.underlying.(interface{ SetURLContextURLs([]string) }); ok {
		p.SetURLContextURLs(urls)
	}
}
