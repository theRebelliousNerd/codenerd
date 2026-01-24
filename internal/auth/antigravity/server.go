package antigravity

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// StartCallbackServer starts a local HTTP server to listen for the OAuth callback.
// Returns the code and state, or an error.
func StartCallbackServer(ctx context.Context, expectedState string) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		state := q.Get("state")
		code := q.Get("code")
		errStr := q.Get("error")

		if state != expectedState {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			errChan <- fmt.Errorf("invalid state received")
			return
		}

		if errStr != "" {
			http.Error(w, "Auth failed: "+errStr, http.StatusBadRequest)
			errChan <- fmt.Errorf("auth failed: %s", errStr)
			return
		}

		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			errChan <- fmt.Errorf("no code received")
			return
		}

		// Success response with auto-close script
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<head><title>Success</title></head>
			<body style="font-family: sans-serif; text-align: center; padding: 50px;">
				<h1 style="color: #4CAF50;">Authentication Successful!</h1>
				<p>You can now close this tab and return to the CLI.</p>
				<script>window.close();</script>
			</body>
			</html>
		`))

		codeChan <- code
	})

	server := &http.Server{Addr: CallbackPort, Handler: mux}

	// Start server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for result or timeout/cancellation
	select {
	case code := <-codeChan:
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return code, nil
	case err := <-errChan:
		server.Close()
		return "", err
	case <-ctx.Done():
		server.Close()
		return "", ctx.Err()
	}
}

// WaitForCallback is an alias for StartCallbackServer for backward compatibility.
func WaitForCallback(ctx context.Context, expectedState string) (string, error) {
	return StartCallbackServer(ctx, expectedState)
}
