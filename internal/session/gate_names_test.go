package session

import "testing"

func TestGateName(t *testing.T) {
	tests := []struct {
		name string
		i    int
		want string
	}{
		{"build", 0, "build"},
		{"test", 1, "test"},
		{"coverage", 2, "coverage"},
		{"critic", 3, "critic"},
		{"unknown negative", -1, "unknown"},
		{"unknown large", 4, "unknown"},
		{"unknown very large", 100, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GateName(tt.i); got != tt.want {
				t.Errorf("GateName(%d) = %q; want %q", tt.i, got, tt.want)
			}
		})
	}
}
