import re

with open("tests/e2e/mcp_virtualstore_integration_test.go", "r") as f:
    content = f.read()

# Add t.Parallel() to all tests
content = re.sub(r'func (TestE2E_[a-zA-Z0-9_]+)\(t \*testing\.T\) \{', r'func \1(t *testing.T) {\n\tt.Parallel()', content)

# Convert one of the tests to a table driven test
table_test = """
// TestE2E_Boundary_MCP_VirtualStore_TableDriven verifies various nil/empty inputs using table driven tests
func TestE2E_Boundary_MCP_VirtualStore_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		serverID string
		tool     string
		args     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "Empty Server ID",
			serverID: "",
			tool:     "test_tool",
			args:     nil,
			wantErr:  true,
		},
		{
			name:     "Empty Tool Name",
			serverID: "test_server",
			tool:     "",
			args:     nil,
			wantErr:  true,
		},
		{
			name:     "Nil Args",
			serverID: "test_server",
			tool:     "test_tool",
			args:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
            manager := mcp.NewMCPClientManager(nil, nil, nil)
            adapter := mcp.NewIntegrationAdapter(manager, tt.serverID)
            _, err := adapter.CallTool(context.Background(), tt.tool, tt.args)
            if (err != nil) != tt.wantErr {
                t.Errorf("CallTool() error = %v, wantErr %v", err, tt.wantErr)
            }
		})
	}
}
"""

content += table_test

with open("tests/e2e/mcp_virtualstore_integration_test.go", "w") as f:
    f.write(content)
