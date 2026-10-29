package system

import (
	"fmt"

	"codenerd/internal/prompt"
)

var (
	_ prompt.KernelContentFingerprinter = (*KernelAdapter)(nil)
	_ prompt.KernelContentFingerprinter = (*kernelCompilationScope)(nil)
)

// ContentFingerprint names the adapted kernel's complete evaluated content
// for the prompt compiler's cache identity (core.RealKernel.ContentFingerprint).
// Only a RealKernel can name its content; any other kernel is an error, not
// an identity that could match a different state.
func (adapter *KernelAdapter) ContentFingerprint() (string, error) {
	if adapter == nil {
		return "", fmt.Errorf("fingerprint prompt kernel snapshot: nil adapter")
	}
	live := adapter.liveRealKernel()
	if live == nil {
		return "", fmt.Errorf("fingerprint prompt kernel snapshot: kernel %T has no RealKernel (nil or unsupported)", adapter.kernel)
	}
	fingerprint, err := live.ContentFingerprint()
	if err != nil {
		return "", fmt.Errorf("fingerprint prompt kernel snapshot: %w", err)
	}
	return fingerprint, nil
}
