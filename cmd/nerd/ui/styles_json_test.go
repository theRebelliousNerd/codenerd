package ui

import (
	"testing"
)

func TestTheme_JSON(t *testing.T) {
	theme := LightTheme()

	data, err := theme.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize theme to JSON: %v", err)
	}

	var newTheme BasicTheme
	if err := newTheme.FromJSON(data); err != nil {
		t.Fatalf("Failed to deserialize theme from JSON: %v", err)
	}

	if newTheme.IsDark() != theme.IsDark() {
		t.Errorf("Expected IsDark=%v, got %v", theme.IsDark(), newTheme.IsDark())
	}
	if newTheme.Background() != theme.Background() {
		t.Errorf("Expected Background=%v, got %v", theme.Background(), newTheme.Background())
	}
	if newTheme.Primary() != theme.Primary() {
		t.Errorf("Expected Primary=%v, got %v", theme.Primary(), newTheme.Primary())
	}
}
