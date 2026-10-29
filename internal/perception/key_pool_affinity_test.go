package perception

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"codenerd/internal/usage"
)

// A conversation keeps one credential, so its prompt cache survives: the
// cache is scoped to the account or workspace a key belongs to.
func TestKeyPool_AConversationKeepsItsCredential(t *testing.T) {
	p := &providerKeyPool{health: make([]keyHealth, 3)}
	ctx := context.Background()
	first, err := p.selectKey(ctx, nil, -1, "session-42")
	if err != nil {
		t.Fatal(err)
	}
	p.release(first, http.StatusOK, 0)
	for i := 0; i < 10; i++ {
		got, err := p.selectKey(ctx, nil, -1, "session-42")
		if err != nil {
			t.Fatal(err)
		}
		if got != first {
			t.Fatalf("call %d of one conversation used key %d, want %d every time", i, got, first)
		}
		p.release(got, http.StatusOK, 0)
	}

	// Many conversations still spread over every key.
	used := map[int]bool{}
	for i := 0; i < 60; i++ {
		got, _ := p.selectKey(ctx, nil, -1, fmt.Sprintf("session-%d", i))
		used[got] = true
		p.release(got, http.StatusOK, 0)
	}
	if len(used) != 3 {
		t.Fatalf("60 conversations used keys %v, want all 3", used)
	}
}

// The preferred key yields when it cannot serve: cooling down after a 429,
// or already tried for this call. The conversation comes back to it after.
func TestKeyPool_AnUnusablePreferredCredentialFallsBack(t *testing.T) {
	p := &providerKeyPool{health: make([]keyHealth, 3)}
	ctx := context.Background()
	pref := affinityIndex("session-7", 3)
	p.health[pref].cooldown = time.Now().Add(time.Hour)
	got, err := p.selectKey(ctx, nil, -1, "session-7")
	if err != nil || got == pref {
		t.Fatalf("a cooling preferred key was used (got %d, pref %d, err %v)", got, pref, err)
	}
	p.release(got, http.StatusOK, 0)
	p.health[pref].cooldown = time.Time{}
	if got, _ := p.selectKey(ctx, map[int]bool{pref: true}, -1, "session-7"); got == pref {
		t.Fatal("a preferred key already tried for this call was chosen again")
	}
	if got, _ := p.selectKey(ctx, nil, -1, "session-7"); got != pref {
		t.Fatalf("after cooling the conversation used key %d, want its own %d", got, pref)
	}
}

// End to end through a pooled client: every call of one session carries one
// credential, so the provider can serve its cached prefix.
func TestKeyPool_EverySessionCallUsesOneCredential(t *testing.T) {
	var mu sync.Mutex
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages/count_tokens" {
			mu.Lock()
			keys = append(keys, r.Header.Get("x-api-key"))
			mu.Unlock()
		}
		poolReply(w, r, false)
	}))
	defer srv.Close()
	client, err := NewClientFromConfig(testPoolConfig(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"session-a", "session-b", "session-c", "session-d"} {
		mu.Lock()
		keys = nil
		mu.Unlock()
		ctx := usage.WithSessionID(context.Background(), session)
		for i := 0; i < 4; i++ {
			if _, err := client.Complete(ctx, "hi"); err != nil {
				t.Fatal(err)
			}
		}
		mu.Lock()
		distinct := map[string]bool{}
		for _, k := range keys {
			distinct[k] = true
		}
		mu.Unlock()
		if len(distinct) != 1 {
			t.Errorf("%s used %d credentials over 4 calls, want one", session, len(distinct))
		}
	}
}
