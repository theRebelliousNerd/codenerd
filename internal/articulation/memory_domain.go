package articulation

// MemoryDomain names which memory a forget operation addresses. The two
// domains have different lifetimes and different owners, and a control must
// never be read as addressing the wider one by default.
type MemoryDomain string

const (
	// MemoryDomainSession (the zero value) is the session scratchpad: the
	// session_note facts a "note" operation records. Every operation a model
	// sends is in this domain.
	MemoryDomainSession MemoryDomain = ""
	// MemoryDomainUserMemory is the user's saved long-term memory: the
	// cold-storage preferences a "remember" instruction writes. Only the host
	// sets it, from an instruction the person typed (the perception adapter's
	// remember and forget actions). It is excluded from JSON, so no model
	// envelope can address durable memory, whatever it emits.
	MemoryDomainUserMemory MemoryDomain = "user_memory"
)
