package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// catalogGenerationServer is a real JSON-RPC HTTP endpoint whose advertised
// catalog changes at each initialize handshake. It can hold one old list
// response so reconnect races exercise the production HTTP transport and the
// real Mangle kernel together.
type catalogGenerationServer struct {
	*httptest.Server

	mu                  sync.Mutex
	generation          int
	failInitialize      bool
	blockResourceGen    int
	blockPromptGen      int
	resourceReached     chan struct{}
	promptReached       chan struct{}
	releaseResourceList chan struct{}
	releasePromptList   chan struct{}
}

func newCatalogGenerationServer(t *testing.T) *catalogGenerationServer {
	t.Helper()
	server := &catalogGenerationServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.Close)
	return server
}

func (server *catalogGenerationServer) handle(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad JSON-RPC request", http.StatusBadRequest)
		return
	}

	server.mu.Lock()
	generation := server.generation
	if request.Method == "initialize" {
		if server.failInitialize {
			server.mu.Unlock()
			http.Error(w, "initialize unavailable", http.StatusServiceUnavailable)
			return
		}
		server.generation++
		generation = server.generation
	}
	block := (<-chan struct{})(nil)
	reached := (chan struct{})(nil)
	if request.Method == "resources/list" && generation == server.blockResourceGen {
		block, reached = server.releaseResourceList, server.resourceReached
	}
	if request.Method == "prompts/list" && generation == server.blockPromptGen {
		block, reached = server.releasePromptList, server.promptReached
	}
	server.mu.Unlock()

	if block != nil {
		select {
		case reached <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
	}

	var result any
	switch request.Method {
	case "initialize":
		result = standardInitializeResult("tools", "resources", "prompts")
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
		return
	case "tools/list":
		result = map[string]any{"tools": []map[string]any{{
			"name":        fmt.Sprintf("offline_tool_%d", generation),
			"description": "cached for offline planning",
			"inputSchema": map[string]any{"type": "object"},
		}}}
	case "resources/list":
		result = map[string]any{"resources": []MCPResource{{
			URI:  fmt.Sprintf("docs://resource-generation-%d", generation),
			Name: fmt.Sprintf("resource generation %d", generation),
		}}}
	case "prompts/list":
		result = map[string]any{"prompts": []MCPPrompt{{
			Name:      fmt.Sprintf("prompt_generation_%d", generation),
			Arguments: []MCPPromptArgument{{Name: fmt.Sprintf("arg_%d", generation), Required: true}},
		}}}
	default:
		result = map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result":  result,
	})
}

func (server *catalogGenerationServer) setFailInitialize(fail bool) {
	server.mu.Lock()
	server.failInitialize = fail
	server.mu.Unlock()
}

func (server *catalogGenerationServer) blockOldList(kind string, generation int) (<-chan struct{}, chan struct{}) {
	server.mu.Lock()
	defer server.mu.Unlock()
	reached := make(chan struct{}, 1)
	release := make(chan struct{})
	switch kind {
	case "resource":
		server.blockResourceGen = generation
		server.resourceReached = reached
		server.releaseResourceList = release
	case "prompt":
		server.blockPromptGen = generation
		server.promptReached = reached
		server.releasePromptList = release
	}
	return reached, release
}

func newGenerationManager(t *testing.T, endpoint string) (*MCPClientManager, *ControlPlane, KernelInterface) {
	t.Helper()
	if RemoteAuthorityKernelForTest == nil {
		t.Fatal("real Mangle kernel fixture is not installed")
	}
	kernel := RemoteAuthorityKernelForTest(t)
	manager := NewMCPClientManager(nil, nil, map[string]MCPServerConfig{
		"srv": {
			ID:       "srv",
			Enabled:  true,
			Protocol: string(ProtocolHTTP),
			BaseURL:  endpoint,
			Timeout:  "10s",
		},
	})
	manager.SetFactEmitter(NewFactEmitter(kernel))
	return manager, NewControlPlane(manager, nil, manager.factEmitter()), kernel
}

func queryCatalogFacts(t *testing.T, kernel KernelInterface, query string) []map[string]any {
	t.Helper()
	rows, err := kernel.Query(query)
	if err != nil {
		t.Fatalf("query %s: %v", query, err)
	}
	return rows
}

func requireCatalogValue(t *testing.T, kernel KernelInterface, predicate, value string, want bool) {
	t.Helper()
	query := fmt.Sprintf(`%s("srv", "%s")`, predicate, value)
	rows := queryCatalogFacts(t, kernel, query)
	if got := len(rows) != 0; got != want {
		t.Fatalf("%s present=%t, want %t (rows=%v)", query, got, want, rows)
	}
}

func TestMCPClientManager_CatalogGenerationRetainsOfflineAndInvalidatesOnReconnect(t *testing.T) {
	server := newCatalogGenerationServer(t)
	manager, plane, kernel := newGenerationManager(t, server.URL)
	bridge := &MCPIntegrationBridge{manager: manager, plane: plane}
	ctx := context.Background()
	if err := bridge.ConnectServer(ctx, "srv"); err != nil {
		t.Fatalf("initial connect: %v", err)
	}
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("discover tools: %v", err)
	}
	initial := plane.catalogFor(ctx, "srv")
	if len(initial.resources) != 1 || initial.resources[0].URI != "docs://resource-generation-1" || len(initial.prompts) != 1 || initial.prompts[0].Name != "prompt_generation_1" {
		t.Fatalf("unexpected generation-1 catalog: %+v", initial)
	}
	if err := manager.Disconnect("srv"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// Offline planning retains all three views of the known catalog: manager
	// metadata, the exact Mangle facts, and the five-minute control-plane cache.
	manager.mu.RLock()
	resourcesAfterDisconnect := append([]MCPResource(nil), manager.resources["srv"]...)
	promptsAfterDisconnect := append([]MCPPrompt(nil), manager.prompts["srv"]...)
	manager.mu.RUnlock()
	if len(resourcesAfterDisconnect) != 1 || resourcesAfterDisconnect[0].URI != "docs://resource-generation-1" {
		t.Fatalf("offline resource cache lost: %+v", resourcesAfterDisconnect)
	}
	if len(promptsAfterDisconnect) != 1 || promptsAfterDisconnect[0].Name != "prompt_generation_1" {
		t.Fatalf("offline prompt cache lost: %+v", promptsAfterDisconnect)
	}
	if cached := plane.cachedCatalog("srv"); cached == nil || len(cached.resources) != 1 || len(cached.prompts) != 1 {
		t.Fatalf("offline ControlPlane catalog lost: %+v", cached)
	}
	requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-1", true)
	requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_1", true)
	oldTool := queryCatalogFacts(t, kernel, `mcp_tool_registered("srv/offline_tool_1", "srv", RegisteredAt)`)
	if len(oldTool) != 1 {
		t.Fatalf("offline tool fact was not retained: %v", oldTool)
	}

	// A failed reconnect attempt leaves the offline catalog intact. The bridge
	// must not eagerly invalidate before knowing a new transport can connect.
	server.setFailInitialize(true)
	if err := bridge.ConnectServer(ctx, "srv"); err == nil {
		t.Fatal("ConnectServer succeeded while initialize was rejected")
	}
	server.setFailInitialize(false)
	if cached := plane.cachedCatalog("srv"); cached == nil {
		t.Fatal("failed reconnect discarded offline ControlPlane catalog")
	}
	manager.mu.RLock()
	resourcesAfterFailedReconnect := append([]MCPResource(nil), manager.resources["srv"]...)
	manager.mu.RUnlock()
	if len(resourcesAfterFailedReconnect) != 1 {
		t.Fatalf("failed reconnect discarded offline manager catalog: %+v", resourcesAfterFailedReconnect)
	}
	requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-1", true)
	requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_1", true)

	if err := bridge.ConnectServer(ctx, "srv"); err != nil {
		t.Fatalf("successful reconnect: %v", err)
	}
	manager.mu.RLock()
	resourcesAfterReconnect := append([]MCPResource(nil), manager.resources["srv"]...)
	promptsAfterReconnect := append([]MCPPrompt(nil), manager.prompts["srv"]...)
	manager.mu.RUnlock()
	if len(resourcesAfterReconnect) != 0 || len(promptsAfterReconnect) != 0 {
		t.Fatalf("new generation retained old manager catalog: resources=%+v prompts=%+v", resourcesAfterReconnect, promptsAfterReconnect)
	}
	if cached := plane.cachedCatalog("srv"); cached != nil {
		t.Fatalf("new generation retained old ControlPlane catalog: %+v", cached)
	}
	requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-1", false)
	requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_1", false)
	// Tool facts are the deliberate offline-planning exception and survive the
	// resource/prompt generation transition until a fresh tools/list replaces
	// them.
	oldTool = queryCatalogFacts(t, kernel, `mcp_tool_registered("srv/offline_tool_1", "srv", RegisteredAt)`)
	if len(oldTool) != 1 {
		t.Fatalf("reconnect removed offline tool fact: %v", oldTool)
	}

	fresh := plane.catalogFor(ctx, "srv")
	if len(fresh.resources) != 1 || fresh.resources[0].URI != "docs://resource-generation-2" || len(fresh.prompts) != 1 || fresh.prompts[0].Name != "prompt_generation_2" {
		t.Fatalf("fresh generation was not published: %+v", fresh)
	}
	requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-2", true)
	requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_2", true)
	requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-1", false)
	requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_1", false)
}

func TestMCPClientManager_LateCatalogResponseCannotOverwriteReconnectGeneration(t *testing.T) {
	for _, kind := range []string{"resource", "prompt"} {
		t.Run(kind, func(t *testing.T) {
			server := newCatalogGenerationServer(t)
			manager, _, kernel := newGenerationManager(t, server.URL)
			ctx := context.Background()
			if err := manager.Connect(ctx, "srv"); err != nil {
				t.Fatalf("initial connect: %v", err)
			}
			var reached <-chan struct{}
			var release chan struct{}
			reached, release = server.blockOldList(kind, 1)
			result := make(chan error, 1)
			go func() {
				if kind == "resource" {
					_, err := manager.DiscoverResources(ctx, "srv")
					result <- err
					return
				}
				_, err := manager.DiscoverPrompts(ctx, "srv")
				result <- err
			}()
			select {
			case <-reached:
			case <-time.After(5 * time.Second):
				t.Fatal("old catalog list did not block in httptest server")
			}

			// Disconnect removes the old manager connection before HTTPTransport
			// waits for the blocked request's read lock to drain. That allows a
			// same-ID connection to become current while the old response is still
			// outstanding.
			disconnected := make(chan error, 1)
			go func() { disconnected <- manager.Disconnect("srv") }()
			deadline := time.After(5 * time.Second)
			for {
				if _, ok := manager.GetServer("srv"); !ok {
					break
				}
				select {
				case <-deadline:
					t.Fatal("disconnect did not retire the old connection")
				case <-time.After(time.Millisecond):
				}
			}
			if err := manager.Connect(ctx, "srv"); err != nil {
				t.Fatalf("reconnect while old list is blocked: %v", err)
			}
			if kind == "resource" {
				if _, err := manager.DiscoverResources(ctx, "srv"); err != nil {
					t.Fatalf("fresh resource discovery: %v", err)
				}
			} else {
				if _, err := manager.DiscoverPrompts(ctx, "srv"); err != nil {
					t.Fatalf("fresh prompt discovery: %v", err)
				}
			}

			close(release)
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("old generation's list response was published")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("old catalog discovery did not finish after release")
			}
			select {
			case err := <-disconnected:
				if err != nil {
					t.Fatalf("retire old transport: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("old transport disconnect did not finish")
			}

			if kind == "resource" {
				requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-2", true)
				requireCatalogValue(t, kernel, "mcp_resource_registered", "docs://resource-generation-1", false)
				manager.mu.RLock()
				got := append([]MCPResource(nil), manager.resources["srv"]...)
				manager.mu.RUnlock()
				if len(got) != 1 || got[0].URI != "docs://resource-generation-2" {
					t.Fatalf("old response replaced fresh resource cache: %+v", got)
				}
			} else {
				requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_2", true)
				requireCatalogValue(t, kernel, "mcp_prompt_registered", "prompt_generation_1", false)
				manager.mu.RLock()
				got := append([]MCPPrompt(nil), manager.prompts["srv"]...)
				manager.mu.RUnlock()
				if len(got) != 1 || got[0].Name != "prompt_generation_2" {
					t.Fatalf("old response replaced fresh prompt cache: %+v", got)
				}
			}
		})
	}
}
