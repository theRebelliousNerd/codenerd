package mcp

import "context"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

func WithRemoteCallIdentity(ctx context.Context, identity RemoteCallIdentity) context.Context {
	return context.WithValue(ctx, remoteIdentityKey{}, &remoteCallState{identity: identity})
}
