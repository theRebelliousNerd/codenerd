package sessionlog

import (
	"net/http"
	"net/url"
	"strings"
)

// Redacted replaces a credential's value in a recorded header or URL. The
// name stays: "which credential was sent" is the useful half.
const Redacted = "[REDACTED]"

var secretQueryKeys = map[string]bool{
	"key": true, "api_key": true, "apikey": true, "access_token": true,
	"token": true, "auth": true, "signature": true, "sig": true,
}

// isSecretHeader reports whether a header carries a credential: the well-known
// auth headers, and anything whose name says key, token, secret, auth or
// cookie.
func isSecretHeader(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "authorization", "proxy-authorization", "x-api-key", "api-key", "x-goog-api-key", "cookie", "set-cookie":
		return true
	}
	for _, frag := range []string{"key", "token", "secret", "auth", "cookie", "credential", "session-id", "account-id"} {
		if strings.Contains(n, frag) {
			return true
		}
	}
	return false
}

// RedactHeaders copies h with every credential header's value replaced.
func RedactHeaders(h http.Header) map[string][]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, vs := range h {
		if isSecretHeader(k) {
			out[k] = []string{Redacted}
			continue
		}
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// RedactURL strips credentials from a URL: userinfo and secret query values
// (Gemini sends its key as ?key=).
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		// An unparseable URL is not echoed: it might hold the secret.
		return Redacted
	}
	if u.User != nil {
		u.User = url.User(redactedURLValue)
	}
	if u.RawQuery != "" {
		q := u.Query()
		changed := false
		for k := range q {
			if secretQueryKeys[strings.ToLower(k)] {
				q.Set(k, redactedURLValue)
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
	}
	return u.String()
}

// redactedURLValue is the in-URL spelling of Redacted: brackets would be
// percent-encoded.
const redactedURLValue = "REDACTED"
