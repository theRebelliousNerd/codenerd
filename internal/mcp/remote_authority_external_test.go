package mcp_test

import (
	"testing"

	"codenerd/internal/mcp"
)

func init() {
	mcp.RemoteAuthorityKernelForTest = func(test *testing.T) mcp.KernelInterface {
		return bootKernel(test)
	}
}
