sed -i 's/func (m \*mockKernel) Retract(fact types.Fact) error {/func (m \*mockKernel) Retract(predicate string) error {/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
sed -i 's/if f.Predicate != fact.Predicate {/if f.Predicate != predicate {/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
sed -i 's/func (m \*mockKernel) RetractFact(fact types.Fact) error { return m.Retract(fact) }/func (m \*mockKernel) RetractFact(fact types.Fact) error { return m.Retract(fact.Predicate) }/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
