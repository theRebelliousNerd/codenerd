package session

import (
	"context"
	"errors"
	"slices"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func TestCompilationContextUsesPermittedToolEnvelope(t *testing.T) {
	for _, requestContext := range []bool{false, true} {
		for _, tc := range []struct {
			name        string
			verb        string
			precompiled *config.EffectiveAgentRuntimeConfig
			factoryErr  error
			want        []string
		}{
			{name: "factory", verb: "/fix", want: []string{"read_file"}},
			{name: "general fallback", want: []string{"read_file"}},
			{name: "resolver error", verb: "/fix", factoryErr: errors.New("unavailable")},
			{name: "specialist", verb: "/fix", precompiled: &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}}, want: []string{"write_file"}},
			{name: "empty specialist", verb: "/fix", precompiled: &config.EffectiveAgentRuntimeConfig{}},
		} {
			t.Run(tc.name+map[bool]string{true: "/request-context", false: "/default-context"}[requestContext], func(t *testing.T) {
				calls := 0
				factory := &MockConfigFactory{ResolveAllowedToolsFunc: func(_ context.Context, intents ...string) ([]string, error) {
					calls++
					wantVerb := tc.verb
					if wantVerb == "" {
						wantVerb = "/general"
					}
					if !slices.Equal(intents, []string{wantVerb}) {
						t.Fatalf("resolved intents=%v, want %s", intents, wantVerb)
					}
					return []string{"read_file"}, tc.factoryErr
				}}
				e := &Executor{configFactory: factory, EffectiveAgentRuntimeConfig: tc.precompiled}
				ctx := t.Context()
				if requestContext {
					ctx = types.WithSessionContext(ctx, &types.SessionContext{})
				}
				cc := e.buildCompilationContext(ctx, perception.Intent{Verb: tc.verb})
				if !slices.Equal(cc.AvailableTools, tc.want) {
					t.Fatalf("prompt envelope=%v, want %v", cc.AvailableTools, tc.want)
				}
				if (tc.precompiled == nil && calls != 1) || (tc.precompiled != nil && calls != 0) {
					t.Fatalf("factory calls=%d with specialist=%v", calls, tc.precompiled != nil)
				}
				if tc.precompiled != nil && len(cc.AvailableTools) != 0 {
					cc.AvailableTools[0] = "changed"
					if !slices.Equal(tc.precompiled.AllowedTools, tc.want) {
						t.Fatal("compilation context aliases specialist permissions")
					}
				}
			})
		}
	}
	if cc := (&Executor{}).buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix"}); len(cc.AvailableTools) != 0 {
		t.Fatal("missing config factory supplied ambient tools")
	}
}
