package typesafe

import (
	"codenerd/internal/config"
)

// FromConfig builds the client for the `typesafe` config section. An absent
// section is Jev off: (nil, nil), so callers test the client, not the config.
// A present section that cannot run, or whose key variable is unset, is an
// error rather than a quietly disabled client.
func FromConfig(sec *config.TypeSafeConfig) (*Client, error) {
	r, err := sec.Resolve()
	if err != nil || r == nil {
		return nil, err
	}
	return NewClient(Options{
		Model:       r.Model,
		BaseURL:     r.BaseURL,
		APIKey:      r.APIKey,
		LongContext: r.LongContext,
		Timeout:     r.Timeout,
	})
}
