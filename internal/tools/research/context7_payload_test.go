package research

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"
	"testing"
)

// The view and raw endpoint serve the same complete Markdown file. The view
// adds repository UI payload; the file itself exceeds the former 8k cut.
func TestContext7FileViewPayload(t *testing.T) {
	const linked = "https://github.com/owner/repo/blob/master/docs/guide.md#response"
	body := "# Echo response guide\nSTART-OF-FILE\n" + strings.Repeat("Full documentation content and response examples.\n", 300) + "END-OF-FILE\n"
	view := "<html><head><script>/*" + strings.Repeat("repository-ui-payload ", 42000) + "*/</script></head><body><nav>UI-NOISE</nav><pre>" + html.EscapeString(body) + "</pre></body></html>"
	base, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch r.URL.Path {
		case "/owner/repo/main/llms.txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "# Echo\n\n[Response guide]("+linked+")\n")
		case "/owner/repo/master/docs/guide.md":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, body)
		case "/view/guide":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, view)
		default:
			http.NotFound(w, r)
		}
	})
	legacy, err := fetchURL(context.Background(), base+"/view/guide")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(legacy, "repository-ui-payload") || !strings.Contains(legacy, "END-OF-FILE") {
		t.Fatal("file-view control did not retain both UI and full documentation")
	}
	t.Logf("file-view control: %d bytes; complete raw document: %d bytes", len(legacy), len(body))
	got, err := Context7Tool().Execute(t.Context(), map[string]any{"topic": "github.com/owner/repo/v4", "max_docs": 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, body) || strings.Contains(got, "UI-NOISE") || strings.Contains(got, "repository-ui-payload") {
		t.Fatal("tool did not preserve the complete file without repository UI")
	}
	if !strings.Contains(got, "## Source: "+base+"/owner/repo/master/docs/guide.md") || !strings.Contains(got, "[Manifest link: "+linked+"]") {
		t.Fatal("tool lost raw document or manifest-link provenance")
	}
	if !strings.Contains(got, "compatibility with any requested dependency version is unverified") {
		t.Fatal("branch discovery must not claim compatibility with the v4 topic")
	}
	if len(got) > len(body)+600 {
		t.Fatalf("tool returned %d bytes for %d file bytes", len(got), len(body))
	}
	if gotRequests := strings.Join(requests(), "\n"); !strings.Contains(gotRequests, "/owner/repo/master/docs/guide.md") || strings.Contains(gotRequests, "/owner/repo/main/docs/guide.md") {
		t.Fatalf("linked master ref changed: %s", gotRequests)
	}
	t.Logf("tool result: %d bytes; complete raw document retained: %d bytes; tokens and model quality not measured", len(got), len(body))
}

func TestContext7LinkedReadURL(t *testing.T) {
	for _, tc := range []struct{ link, want string }{
		{"https://github.com/o/r/blob/master/docs/guide.md#part", "https://raw.githubusercontent.com/o/r/master/docs/guide.md"},
		{"https://github.com/o/r/blob/release%2Fv4/docs/guide%20name.md", "https://raw.githubusercontent.com/o/r/release%2Fv4/docs/guide%20name.md"},
		{"https://github.com/o/r/blob/123abc/README.MD", "https://raw.githubusercontent.com/o/r/123abc/README.MD"},
		{"https://github.com/o/r/blob/master/guide.md?token=opaque", ""},
		{"https://github.com/o/r/blob/master/guide.md?", ""},
		{"http://github.com/o/r/blob/master/guide.md", ""},
		{"https://github.com:443/o/r/blob/master/guide.md", ""},
		{"https://github.com.evil.test/o/r/blob/master/guide.md", ""},
		{"https://github.com/o/r/tree/master/docs", ""},
		{"https://github.com/o/r/blob/master", ""},
		{"https://github.com/o/r/blob/master/image.png", ""},
		{"https://docs.example/guide.md", ""},
	} {
		want := tc.want
		if want == "" {
			want = tc.link
		}
		if got := context7LinkedReadURL(tc.link); got != want {
			t.Errorf("read URL(%q) = %q, want %q", tc.link, got, want)
		}
	}
}

func TestContext7LinkedFileMissing(t *testing.T) {
	base, requests := context7RefFixture(t, hostGitHub, func(w http.ResponseWriter, r *http.Request, _ string) {
		http.NotFound(w, r)
	})
	docs, err := parseLlmsTxt(t.Context(), base+"/manifest.txt", "[Missing](https://github.com/owner/repo/blob/master/docs/missing.md)", 1)
	if err != nil || len(docs) != 0 {
		t.Fatalf("missing raw file produced docs=%v err=%v", docs, err)
	}
	if got := requests(); len(got) != 1 || got[0] != "/owner/repo/master/docs/missing.md" {
		t.Fatalf("missing raw file retried a different ref or representation: %v", got)
	}
}
