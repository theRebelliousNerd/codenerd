sed -i 's/cfg.EnableSafetyGate = false/cfg.EnableSafetyGate = false\n\tcfg.ToolTimeout = 1 * time.Second/g' tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go
