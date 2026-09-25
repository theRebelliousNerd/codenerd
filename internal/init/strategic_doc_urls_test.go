package init

import (
	"reflect"
	"testing"
)

// The strategic knowledge pass grounds on the documentation of the project's
// language and framework. A technology named twice (detection reports the
// language as the framework too, in any case) must not spend the URL budget
// twice, and an unknown technology contributes nothing.
func TestStrategicDocURLs_ShouldSendEachDocumentationURLOnce(t *testing.T) {
	goDocs := []string{"https://go.dev/doc/", "https://pkg.go.dev/std", "https://go.dev/blog/"}

	got := strategicDocURLs(ProjectProfile{Language: "Go", Framework: "go"})
	if !reflect.DeepEqual(got, goDocs) {
		t.Fatalf("language and framework both Go: got %v, want %v", got, goDocs)
	}

	got = strategicDocURLs(ProjectProfile{Language: "go", Framework: "bubbletea"})
	want := append(append([]string{}, goDocs...),
		"https://github.com/charmbracelet/bubbletea",
		"https://pkg.go.dev/github.com/charmbracelet/bubbletea")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("go + bubbletea: got %v, want %v", got, want)
	}

	if got := strategicDocURLs(ProjectProfile{Language: "cobol"}); len(got) != 0 {
		t.Fatalf("an unknown language produced URLs: %v", got)
	}
}
