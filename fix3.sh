sed -i 's/func (m \*mockTransducer) Transduce(ctx context.Context, input string) (perception.Intent, error) {/func (m \*mockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
cat << 'INNEREOF' >> tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
func (m *mockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) { return m.intent, m.err }
func (m *mockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, []string, error) { return m.intent, nil, m.err }
INNEREOF
