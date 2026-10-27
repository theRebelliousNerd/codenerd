package usage

import "context"

type credentialAliasKey struct{}

// WithCredentialAlias attaches a non-secret provider/credential alias to usage.
func WithCredentialAlias(ctx context.Context, alias string) context.Context {
	return context.WithValue(ctx, credentialAliasKey{}, alias)
}
func CredentialAliasFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(credentialAliasKey{}).(string)
	return value
}
