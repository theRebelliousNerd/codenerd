package mangle

// Observers over the LSP index for tests; the server answers editors through
// handleRequest (definition, references, completion, published diagnostics).

// GetDefinitions returns all definitions for a symbol.
func (s *LSPServer) GetDefinitions(symbol string) []Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Definition(nil), s.definitions[symbol]...)
}

// GetReferences returns all references to a symbol.
func (s *LSPServer) GetReferences(symbol string) []Reference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Reference(nil), s.references[symbol]...)
}
