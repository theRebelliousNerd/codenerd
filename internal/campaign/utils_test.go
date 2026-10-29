package campaign

import (
	"reflect"
	"testing"
)

func TestChunkText(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   []string
	}{
		{
			name:   "empty string",
			text:   "",
			maxLen: 10,
			want:   nil,
		},
		{
			name:   "shorter than maxLen",
			text:   "hello",
			maxLen: 10,
			want:   []string{"hello"},
		},
		{
			name:   "defaults to 2000 if maxLen <= 0",
			text:   "hello",
			maxLen: 0,
			want:   []string{"hello"},
		},
		{
			name:   "splits on paragraph",
			text:   "hello\n\nworld",
			maxLen: 8,
			want:   []string{"hello\n\n", "world"},
		},
		{
			name:   "splits on newline",
			text:   "hello\nworld",
			maxLen: 8,
			want:   []string{"hello\n", "world"},
		},
		{
			name:   "splits on space",
			text:   "hello world",
			maxLen: 8,
			want:   []string{"hello", "world"},
		},
		{
			name:   "hard split when no boundaries",
			text:   "helloworld",
			maxLen: 5,
			want:   []string{"hello", "world"},
		},
		{
			name:   "multiple splits",
			text:   "a b c d e",
			maxLen: 2,
			want:   []string{"a", "b", "c", "d", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chunkText(tt.text, tt.maxLen)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("chunkText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeCampaignID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "empty string",
			id:   "",
			want: "",
		},
		{
			name: "valid id",
			id:   "valid_id-123",
			want: "valid_id-123",
		},
		{
			name: "removes leading slash",
			id:   "/valid_id",
			want: "valid_id",
		},
		{
			name: "replaces invalid chars",
			id:   "invalid!@#id",
			want: "invalid_id",
		},
		{
			name: "collapses consecutive invalid chars",
			id:   "invalid   id",
			want: "invalid_id",
		},
		{
			name: "leading invalid chars",
			id:   "!@#invalid",
			want: "_invalid",
		},
		{
			name: "trailing invalid chars",
			id:   "invalid!@#",
			want: "invalid_",
		},
		{
			name: "leading slash and invalid chars",
			id:   "/invalid!@#id",
			want: "invalid_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCampaignID(tt.id)
			if got != tt.want {
				t.Errorf("sanitizeCampaignID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPathWithinWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		target    string
		want      bool
	}{
		{
			name:      "empty workspace",
			workspace: "",
			target:    "/some/path",
			want:      true,
		},
		{
			name:      "empty target",
			workspace: "/some/workspace",
			target:    "",
			want:      true,
		},
		{
			name:      "target inside workspace",
			workspace: "/workspace",
			target:    "/workspace/target",
			want:      true,
		},
		{
			name:      "target is workspace",
			workspace: "/workspace",
			target:    "/workspace",
			want:      true,
		},
		{
			name:      "target outside workspace",
			workspace: "/workspace",
			target:    "/outside/target",
			want:      false,
		},
		{
			name:      "directory traversal outside",
			workspace: "/workspace",
			target:    "/workspace/../outside",
			want:      false,
		},
		{
			name:      "similar prefix but different directory",
			workspace: "/workspace",
			target:    "/workspace-other",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathWithinWorkspace(tt.workspace, tt.target)
			if got != tt.want {
				t.Errorf("isPathWithinWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupportedDocExt(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "markdown",
			path: "doc.md",
			want: true,
		},
		{
			name: "uppercase extension",
			path: "doc.MD",
			want: true,
		},
		{
			name: "text file",
			path: "doc.txt",
			want: true,
		},
		{
			name: "go file",
			path: "main.go",
			want: true,
		},
		{
			name: "unsupported extension",
			path: "program.exe",
			want: false,
		},
		{
			name: "no extension",
			path: "README",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSupportedDocExt(tt.path)
			if got != tt.want {
				t.Errorf("isSupportedDocExt() = %v, want %v", got, tt.want)
			}
		})
	}
}
