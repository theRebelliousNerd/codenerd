package mcp_test

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/mcp"
	"codenerd/internal/system"
)

func init() {
	mcp.RemoteAuthorityKernelForTest = func(test *testing.T) mcp.KernelInterface {
		return bootKernel(test)
	}
	mcp.RemoteAuthorityShardedKernelForTest = func(test *testing.T) mcp.KernelInterface {
		cortex, err := system.NewDomainCortex(test.TempDir())
		if err != nil {
			test.Fatalf("boot production sharded Cortex: %v", err)
		}
		var kernel core.Kernel = cortex
		return &kernelUnderTest{kernel: kernel}
	}
}
