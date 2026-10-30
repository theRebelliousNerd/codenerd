package research

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestContext7Document_Locators(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  context7Document
		base string
	}{
		{"requested fallback", context7Document{requestedURL: "https://raw.example/o/r/master/llms.txt"}, "https://raw.example/o/r/master/llms.txt"},
		{"observed redirect", context7Document{requestedURL: "https://raw.example/o/r/master/llms.txt", documentURL: "https://docs.example/release/llms.txt"}, "https://docs.example/release/llms.txt"},
		{"observed base", context7Document{documentURL: "https://docs.example/release/llms.txt", baseURI: "https://user:password@docs.example/manual/"}, "https://docs.example/manual/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.doc.linkBase(); got != tc.base {
				t.Fatalf("linkBase = %q, want %q", got, tc.base)
			}
			if got := resolveLink(tc.doc.linkBase(), "./guide.md"); got != resolveLink(tc.base, "./guide.md") {
				t.Errorf("resolved guide = %q", got)
			}
		})
	}
}

// This fixture keeps real browser reads and records each document request.
// Only raw repository URL construction is redirected to the local server.
func context7RefFixture(t *testing.T, host string, handler func(http.ResponseWriter, *http.Request, string)) (string, func() []string) {
	t.Helper()
	withBoundResearchRuntime(t)
	var mu sync.Mutex
	var requests []string
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		requests = append(requests, r.URL.RequestURI())
		mu.Unlock()
		handler(w, r, ts.URL)
	}))
	t.Cleanup(ts.Close)
	prevGitHub, prevCodeberg := rawGitHubFileURL, rawCodebergFileURL
	rawGitHubFileURL = func(owner, repo, ref, path string) string {
		if host != hostGitHub {
			t.Errorf("Codeberg repository routed through GitHub")
		}
		return ts.URL + "/" + owner + "/" + repo + "/" + ref + "/" + path
	}
	rawCodebergFileURL = func(owner, repo, ref, path string) string {
		if host != hostCodeberg {
			t.Errorf("GitHub repository routed through Codeberg")
		}
		return ts.URL + "/" + owner + "/" + repo + "/raw/branch/" + ref + "/" + path
	}
	t.Cleanup(func() { rawGitHubFileURL, rawCodebergFileURL = prevGitHub, prevCodeberg })
	return ts.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), requests...)
	}
}

func context7RefBody(marker string) string {
	return marker + "\n" + strings.Repeat("Documentation body specific to this fixture. ", 4)
}

func TestContext7Ref_ManifestContinuity(t *testing.T) {
	for _, tc := range []struct {
		name, host, ref string
		competing       bool
	}{
		{"main control", hostGitHub, "main", true},
		{"master only", hostGitHub, "master", false},
		{"master wins competing main body", hostGitHub, "master", true},
		{"Codeberg master retains owning host", hostCodeberg, "master", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := "/owner/repo/"
			repo := "owner/repo"
			if tc.host == hostCodeberg {
				prefix += "raw/branch/"
				repo = hostCodeberg + "/" + repo
			}
			manifest := prefix + tc.ref + "/llms.txt"
			guide := prefix + tc.ref + "/docs/guide.md"
			body := context7RefBody("SELECTED-" + tc.ref)
			base, requests := context7RefFixture(t, tc.host, func(w http.ResponseWriter, r *http.Request, _ string) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				switch r.URL.Path {
				case manifest:
					fmt.Fprint(w, "[Guide](./docs/guide.md)")
				case guide:
					fmt.Fprint(w, body)
				default:
					if tc.competing && strings.HasSuffix(r.URL.Path, "/docs/guide.md") {
						fmt.Fprint(w, context7RefBody("WRONG-BRANCH"))
					} else {
						http.NotFound(w, r)
					}
				}
			})
			result, err := Context7Tool().Execute(context.Background(), map[string]any{"topic": "fixture", "repo": repo, "max_docs": 1})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(result, "## Source: "+base+guide+"\n\n"+body) || strings.Contains(result, "WRONG-BRANCH") {
				t.Fatalf("wrong content or source: %s", result)
			}
			want := []string{prefix + "main/llms.txt"}
			if tc.ref == "master" {
				want = append(want, manifest)
			}
			want = append(want, guide)
			if got := requests(); !reflect.DeepEqual(got, want) {
				t.Errorf("requests = %v, want %v", got, want)
			}
		})
	}
}

func TestContext7Ref_CommonFallback(t *testing.T) {
	paths := []string{"README.md", "docs/README.md", "documentation/README.md", "docs/getting-started.md", "docs/quickstart.md", "GETTING_STARTED.md"}
	for _, tc := range []struct {
		name, selected string
		maxDocs        int
		emptyManifest  bool
	}{
		{"master README with no manifests", "master", 1, false},
		{"first main document prevents mixed refs", "main", 2, false},
		{"empty manifest still falls back", "master", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := context7RefBody("FALLBACK-" + tc.selected)
			base, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				switch r.URL.Path {
				case "/owner/repo/main/llms.txt":
					if tc.emptyManifest {
						fmt.Fprint(w, "[Missing](docs/absent.md)")
						return
					}
				case "/owner/repo/" + tc.selected + "/README.md":
					fmt.Fprint(w, body)
					return
				case "/owner/repo/master/docs/README.md":
					fmt.Fprint(w, context7RefBody("MUST-NOT-MIX"))
					return
				}
				http.NotFound(w, r)
			})
			result, err := executeContext7(context.Background(), map[string]any{"topic": "fixture", "repo": "owner/repo", "max_docs": tc.maxDocs})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(result, "## Source: "+base+"/owner/repo/"+tc.selected+"/README.md\n\n"+body) || strings.Contains(result, "MUST-NOT-MIX") || strings.Count(result, "## Source:") != 1 {
				t.Fatalf("wrong fallback content or provenance: %s", result)
			}
			want := []string{"/owner/repo/main/llms.txt"}
			if tc.emptyManifest {
				want = append(want, "/owner/repo/main/docs/absent.md")
			} else {
				want = append(want, "/owner/repo/master/llms.txt", "/owner/repo/main/.llms.txt")
			}
			for _, path := range paths {
				want = append(want, "/owner/repo/main/"+path)
			}
			if tc.selected == "master" {
				want = append(want, "/owner/repo/master/README.md")
			}
			if got := requests(); !reflect.DeepEqual(got, want) {
				t.Errorf("requests = %v, want %v", got, want)
			}
		})
	}
}

func TestContext7Ref_RedirectAndObservedBase(t *testing.T) {
	body := context7RefBody("REDIRECTED-GUIDE")
	base, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch r.URL.Path {
		case "/owner/repo/main/llms.txt":
			http.Redirect(w, r, "/release/llms.txt", http.StatusFound)
		case "/release/llms.txt":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><head><base href="/manual/"></head><body><pre>[Guide](guide.md)</pre></body></html>`)
		case "/manual/guide.md":
			http.Redirect(w, r, "/published/guide.md", http.StatusFound)
		case "/published/guide.md":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, body)
		default:
			http.NotFound(w, r)
		}
	})
	result, err := executeContext7(context.Background(), map[string]any{"topic": "fixture", "repo": "owner/repo", "max_docs": 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "## Source: "+base+"/published/guide.md\n\n"+body) {
		t.Fatalf("wrong redirected provenance: %s", result)
	}
	want := []string{"/owner/repo/main/llms.txt", "/release/llms.txt", "/manual/guide.md", "/published/guide.md"}
	if got := requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestContext7Ref_AbsoluteCredentialStripping(t *testing.T) {
	body := context7RefBody("ABSOLUTE-GUIDE")
	base, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, base string) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		switch r.URL.Path {
		case "/owner/repo/main/llms.txt":
			url := strings.Replace(base, "http://", "http://user:password@", 1) + "/external/guide.md?edition=one"
			fmt.Fprintf(w, "[Guide](%s)", url)
		case "/external/guide.md":
			if r.Header.Get("Authorization") != "" {
				t.Error("document credentials sent to server")
			}
			fmt.Fprint(w, body)
		default:
			http.NotFound(w, r)
		}
	})
	result, err := executeContext7(context.Background(), map[string]any{"topic": "fixture", "repo": "owner/repo", "max_docs": 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "## Source: "+base+"/external/guide.md?edition=one\n\n"+body) || strings.Contains(result, "password") {
		t.Fatalf("wrong absolute provenance: %s", result)
	}
	want := []string{"/owner/repo/main/llms.txt", "/external/guide.md?edition=one"}
	if got := requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestContext7Ref_CanceledCallerStopsDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := executeContext7(ctx, map[string]any{"topic": "fixture", "repo": "owner/repo"})
	if result != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("execute = %q, %v; want empty result and context.Canceled", result, err)
	}
}

func TestContext7Ref_CancellationDuringManifestStopsFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
		cancel()
		<-r.Context().Done()
	})
	result, err := executeContext7(ctx, map[string]any{"topic": "fixture", "repo": "owner/repo"})
	if result != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("execute = %q, %v; want empty result and context.Canceled", result, err)
	}
	if got, want := requests(), []string{"/owner/repo/main/llms.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestContext7Ref_NoDocumentsHasBoundedAttempts(t *testing.T) {
	_, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
		// Error bodies must not become documentation, even when substantial.
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, context7RefBody("ERROR-PAGE-MUST-NOT-BE-KNOWLEDGE"))
	})
	assertNoContext7Docs(t, map[string]any{"topic": "fixture", "repo": "owner/repo"}, "fixture")
	want := []string{"/owner/repo/main/llms.txt", "/owner/repo/master/llms.txt", "/owner/repo/main/.llms.txt"}
	for _, ref := range []string{"main", "master"} {
		for _, path := range []string{"README.md", "docs/README.md", "documentation/README.md", "docs/getting-started.md", "docs/quickstart.md", "GETTING_STARTED.md"} {
			want = append(want, "/owner/repo/"+ref+"/"+path)
		}
	}
	if got := requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}
