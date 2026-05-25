package mcp

// ServersForTest allows tests outside the mcp package to access the unexported servers map.
func (m *MCPClientManager) ServersForTest() map[string]*MCPServerConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers
}
