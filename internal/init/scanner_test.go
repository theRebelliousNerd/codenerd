package init

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectEntryPoints(t *testing.T) {
	// Create temporary workspace
	tmpDir, err := os.MkdirTemp("", "nerd_init_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	conf := InitConfig{Workspace: tmpDir}
	init := &Initializer{config: conf}

	t.Run("Go Standard Layout", func(t *testing.T) {
		// main.go at root
		os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
		// cmd/app/main.go
		os.MkdirAll(filepath.Join(tmpDir, "cmd", "app"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "cmd", "app", "main.go"), []byte("package main\nfunc main() {}"), 0644)
		// internal/lib.go (should not be detected)
		os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "internal", "lib.go"), []byte("package internal"), 0644)

		eps := init.detectEntryPoints()
		assert.Contains(t, eps, "main.go")
		expectedCmd := filepath.Join("cmd", "app", "main.go")
		assert.Contains(t, eps, expectedCmd)
		assert.NotContains(t, eps, filepath.Join("internal", "lib.go"))
	})

	t.Run("Python Script", func(t *testing.T) {
		os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte("if __name__ == \"__main__\":\n    pass"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "lib.py"), []byte("def func(): pass"), 0644)

		eps := init.detectEntryPoints()
		assert.Contains(t, eps, "script.py")
		assert.NotContains(t, eps, "lib.py")
	})

	t.Run("Node Package", func(t *testing.T) {
		pkgJson := `{"main": "index.js", "bin": "cli.js", "scripts": {"start": "node server.js"}}`
		os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJson), 0644)
		// Only files that exist are returned, so create them
		os.WriteFile(filepath.Join(tmpDir, "index.js"), []byte(""), 0644)
		os.WriteFile(filepath.Join(tmpDir, "cli.js"), []byte(""), 0644)
		os.WriteFile(filepath.Join(tmpDir, "server.js"), []byte(""), 0644)

		eps := init.detectEntryPoints()
		assert.Contains(t, eps, "index.js")
		assert.Contains(t, eps, "cli.js")
		assert.Contains(t, eps, "server.js")
	})
}
