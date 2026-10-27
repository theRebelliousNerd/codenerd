package broker

import "codenerd/internal/types"

// InputWindow is the input cap a client chain reports, in tokens: the smallest
// positive types.InputWindowReporter answer on any layer Walk reaches, 0 when
// no layer reports one.
//
// The smallest, because every layer stands between the caller and the wire: a
// request that fits an outer layer's window and not an inner one's is refused
// by the inner one. A credential-pool wrapper answers through the client it has
// selected (Unwrap), and every client of a pool is built from one slot config,
// so a pooled slot reports the slot's cap.
func InputWindow(client types.LLMClient) int {
	window := 0
	Walk(client, func(layer types.LLMClient) bool {
		reporter, ok := layer.(types.InputWindowReporter)
		if !ok {
			return true
		}
		if n := reporter.InputWindowTokens(); n > 0 && (window == 0 || n < window) {
			window = n
		}
		return true
	})
	return window
}
