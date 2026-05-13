sed -i 's/func (m \*mockLLMClient) CompleteWithTools(ctx context.Context, prompt string, input string, tools \[\]string) (\*types.LLMToolResponse, error) {/func (m \*mockLLMClient) CompleteWithTools(ctx context.Context, prompt string, input string, tools \[\]types.ToolDefinition) (\*types.LLMToolResponse, error) {/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
cat << 'INNEREOF' >> tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) { return "mock", nil }
func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) { return "mock", nil }
INNEREOF
