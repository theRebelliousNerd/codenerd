package chat

import (
	"codenerd/internal/config"
	"testing"
)

func TestCommandCategoryString_KnownAndUnknown(t *testing.T) {
	if got := CategoryCore.String(); got != "Core" {
		t.Errorf("CategoryCore.String() = %q, want %q", got, "Core")
	}
	if got := CommandCategory(99).String(); got != "Unknown" {
		t.Errorf("CommandCategory(99).String() = %q, want %q", got, "Unknown")
	}
}

func TestGetCommandsByCategory_FiltersShowInHelp(t *testing.T) {
	original := append([]CommandInfo(nil), CommandRegistry...)
	t.Cleanup(func() {
		CommandRegistry = original
	})

	CommandRegistry = append(CommandRegistry, CommandInfo{
		Name:       "/hidden",
		Category:   CategoryCore,
		ShowInHelp: false,
	})

	commands := GetCommandsByCategory(CategoryCore)
	if containsCommand(commands, "/hidden") {
		t.Fatalf("GetCommandsByCategory returned a hidden command")
	}
}

func TestGetCommandsForLevel_IncludesExpectedCategories(t *testing.T) {
	tests := []struct {
		name    string
		level   config.ExperienceLevel
		want    []string
		notWant []string
	}{
		{
			name:    "beginner",
			level:   config.ExperienceBeginner,
			want:    []string{"/help"},
			notWant: []string{"/read", "/query", "/logic", "/config"},
		},
		{
			name:    "intermediate",
			level:   config.ExperienceIntermediate,
			want:    []string{"/help", "/read"},
			notWant: []string{"/query", "/logic", "/config"},
		},
		{
			name:    "advanced",
			level:   config.ExperienceAdvanced,
			want:    []string{"/help", "/read", "/query"},
			notWant: []string{"/logic", "/config"},
		},
		{
			name:    "expert",
			level:   config.ExperienceExpert,
			want:    []string{"/help", "/read", "/query", "/logic", "/config"},
			notWant: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := GetCommandsForLevel(tt.level)
			for _, name := range tt.want {
				if !containsCommand(commands, name) {
					t.Errorf("expected command %q to be present", name)
				}
			}
			for _, name := range tt.notWant {
				if containsCommand(commands, name) {
					t.Errorf("expected command %q to be absent", name)
				}
			}
		})
	}
}

func TestFindCommand_ByNameAndAlias(t *testing.T) {
	cmd := FindCommand("/help")
	if cmd == nil || cmd.Name != "/help" {
		t.Fatalf("FindCommand(\"/help\") returned %v", cmd)
	}

	alias := FindCommand("/h")
	if alias == nil || alias.Name != "/help" {
		t.Fatalf("FindCommand(\"/h\") returned %v", alias)
	}

	if FindCommand("/does-not-exist") != nil {
		t.Fatal("expected nil for unknown command")
	}
}

func TestGetAllCategories_ContainsExpectedCommands(t *testing.T) {
	categories := GetAllCategories()
	checks := []struct {
		category CommandCategory
		command  string
	}{
		{CategoryCore, "/help"},
		{CategoryBasic, "/read"},
		{CategoryAdvanced, "/query"},
		{CategoryExpert, "/logic"},
		{CategorySystem, "/config"},
	}

	for _, check := range checks {
		cmds, ok := categories[check.category]
		if !ok {
			t.Fatalf("missing category %v", check.category)
		}
		if !containsCommand(cmds, check.command) {
			t.Fatalf("category %v missing command %q", check.category, check.command)
		}
	}
}

func containsCommand(commands []CommandInfo, name string) bool {
	for _, cmd := range commands {
		if cmd.Name == name {
			return true
		}
	}
	return false
}
