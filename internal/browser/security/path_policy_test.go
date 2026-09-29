package security

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Upload confinement tests are adapted from BrowserNERD's Apache-2.0
// browser-act contract and exercise codeNERD's shared secret-path policy.

func writeUploadFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("upload fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveForUploadConfinesWorkspaceFiles(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	file := filepath.Join(workspace, "imports", "data.csv")
	writeUploadFixture(t, file)
	writeUploadFixture(t, filepath.Join(workspace, "data.csv"))
	want, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside", "data.csv")
	writeUploadFixture(t, outside)
	siblingFile := filepath.Join(parent, "workspace-sibling", "data.csv")
	writeUploadFixture(t, siblingFile)
	policy, err := NewPathPolicy(workspace, []string{filepath.Dir(outside)})
	if err != nil {
		t.Fatal(err)
	}
	for _, requested := range []string{file, filepath.Join("imports", "data.csv")} {
		got, err := policy.ResolveForUpload(requested)
		if err != nil || got != want {
			t.Errorf("ResolveForUpload(%q) = %q, %v; want %q, nil", requested, got, err, want)
		}
	}
	if got, err := policy.ResolveForUpload(outside); err == nil || got != "" {
		t.Fatalf("upload accepted a file outside the workspace in a writable root: %q, %v", got, err)
	}
	for _, tc := range []struct {
		name string
		path string
	}{
		{"outside", outside},
		{"sibling prefix", siblingFile},
		{"parent traversal", "../workspace-sibling/data.csv"},
		{"backslash traversal", `..\workspace-sibling\data.csv`},
		{"absolute traversal", workspace + string(filepath.Separator) + ".." + string(filepath.Separator) + "workspace-sibling" + string(filepath.Separator) + "data.csv"},
		{"internal traversal", "imports/../data.csv"},
		{"directory", workspace},
		{"missing", "missing.csv"},
		{"empty", ""},
		{"whitespace", "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := policy.ResolveForUpload(tc.path); err == nil || got != "" {
				t.Errorf("ResolveForUpload(%q) = %q, %v; want empty path and refusal", tc.path, got, err)
			}
		})
	}
	var nilPolicy *PathPolicy
	if got, err := nilPolicy.ResolveForUpload("data.csv"); err == nil || got != "" {
		t.Errorf("nil policy accepted upload: %q, %v", got, err)
	}
}

func TestResolveForUploadRefusesSecretPaths(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		".env", ".ENV", "production.env", "client_secret_x.json",
		filepath.Join(".credentials", "key.json"),
		filepath.Join(".credentials", "nested", "key.json"),
		filepath.Join(".nerd", "config.json"),
		filepath.Join("certs", "server.pem"), ".hidden.txt",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(workspace, name)
			writeUploadFixture(t, file)
			for _, requested := range []string{name, file} {
				if got, err := policy.ResolveForUpload(requested); err == nil || got != "" {
					t.Errorf("secret upload %q = %q, %v; want empty path and refusal", requested, got, err)
				}
			}
		})
	}
}

func TestResolveForUploadResolvesSymlinks(t *testing.T) {
	workspace := t.TempDir()
	normal := filepath.Join(workspace, "data.csv")
	secret := filepath.Join(workspace, "client_secret_x.json")
	hidden := filepath.Join(workspace, ".private", "data.csv")
	outside := filepath.Join(t.TempDir(), "data.csv")
	for _, file := range []string{normal, secret, hidden, outside} {
		writeUploadFixture(t, file)
	}
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		target string
		child  string
		allow  bool
	}{
		{"inside.csv", normal, "", true},
		{"escape.csv", outside, "", false},
		{"broken.csv", filepath.Join(workspace, "missing.csv"), "", false},
		{"disguised.csv", secret, "", false},
		{"disguised-hidden.csv", hidden, "", false},
		{"client_secret_alias.json", normal, "", false},
		{".hidden.csv", normal, "", false},
		{"inside-dir", workspace, "data.csv", true},
		{"escape-dir", filepath.Dir(outside), "data.csv", false},
		{".hidden-dir", workspace, "data.csv", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link := filepath.Join(workspace, tc.name)
			if err := os.Symlink(tc.target, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			got, err := policy.ResolveForUpload(filepath.Join(link, tc.child))
			if !tc.allow {
				if err == nil || got != "" {
					t.Fatalf("symlink upload = %q, %v; want empty path and refusal", got, err)
				}
				return
			}
			want, resolveErr := filepath.EvalSymlinks(filepath.Join(tc.target, tc.child))
			if resolveErr != nil {
				t.Fatal(resolveErr)
			}
			if err != nil || got != want {
				t.Fatalf("symlink upload = %q, %v; want %q, nil", got, err, want)
			}
		})
	}
}

func TestResolveForUploadResolvesWorkspaceAlias(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	file := filepath.Join(workspace, "data.csv")
	writeUploadFixture(t, file)
	alias := filepath.Join(parent, "workspace-alias")
	if err := os.Symlink(workspace, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	policy, err := NewPathPolicy(alias, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, requested := range []string{file, filepath.Join(alias, "data.csv"), "data.csv"} {
		got, err := policy.ResolveForUpload(requested)
		if err != nil || got != want {
			t.Errorf("workspace alias upload %q = %q, %v; want %q, nil", requested, got, err, want)
		}
	}
	hidden := filepath.Join(workspace, ".hidden.csv")
	if err := os.Symlink(file, hidden); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, requested := range []string{hidden, filepath.Join(alias, ".hidden.csv"), ".hidden.csv"} {
		if got, err := policy.ResolveForUpload(requested); err == nil || got != "" {
			t.Errorf("workspace alias hid a dot path %q: %q, %v", requested, got, err)
		}
	}
}

func TestResolveForUploadRefusesNonRegularFiles(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		for _, name := range []string{"NUL", "CON", "NUL.txt"} {
			for _, requested := range []string{name, filepath.Join(workspace, name)} {
				if got, err := policy.ResolveForUpload(requested); err == nil || got != "" {
					t.Errorf("device upload %q = %q, %v; want empty path and refusal", requested, got, err)
				}
			}
		}
		return
	}
	socket := filepath.Join(workspace, "upload.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("filesystem sockets unavailable: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if got, err := policy.ResolveForUpload(socket); err == nil || got != "" {
		t.Fatalf("socket upload = %q, %v; want empty path and refusal", got, err)
	}
}

func TestResolveForUploadRefusesAlternateDataStreams(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("alternate data streams require Windows")
	}
	workspace := t.TempDir()
	file := filepath.Join(workspace, "data.csv:private")
	writeUploadFixture(t, file)
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := policy.ResolveForUpload(file); err == nil || got != "" {
		t.Fatalf("alternate data stream upload = %q, %v; want empty path and refusal", got, err)
	}
}

func TestPathPolicyConfinesBrowserWrites(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workspace, ".nerd", "browser", "screenshots", "shot.png")
	got, err := policy.ResolveForWrite("shot.png", policy.DefaultRoot(), "default.png")
	if err != nil {
		t.Fatalf("ResolveForWrite(valid) error = %v", err)
	}
	if got != want {
		t.Fatalf("ResolveForWrite(valid) = %q, want %q", got, want)
	}
	if _, err := policy.ResolveForWrite(filepath.Join(workspace, "outside.txt"), "", ""); err == nil {
		t.Fatal("path policy accepted an output outside writable roots")
	}
}

func TestPathPolicyDefaultRootsIncludeSnapshots(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"screenshots", "traces", "snapshots"} {
		path := filepath.Join(workspace, ".nerd", "browser", dir, "artifact.mg")
		if _, err := policy.ResolveForWrite(path, "", ""); err != nil {
			t.Fatalf("default policy should allow %s (%q): %v", dir, path, err)
		}
	}
	rejected := filepath.Join(workspace, ".nerd", "browser", "elsewhere", "artifact.mg")
	if _, err := policy.ResolveForWrite(rejected, "", ""); err == nil {
		t.Fatalf("default policy should reject path outside writable roots: %q", rejected)
	}
}

func TestPathPolicyRejectsExistingSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "artifacts")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	policy, err := NewPathPolicy(workspace, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.ResolveForWrite(filepath.Join(link, "evidence.json"), "", ""); err == nil {
		t.Fatal("path policy accepted a symlink escape")
	}
}

func TestPrivateBrowserArtifactPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "artifact.txt")
	if err := WritePrivateFile(path, []byte("evidence")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "evidence" {
		t.Fatalf("private artifact read = %q, %v", data, err)
	}
	privateDir, err := IsPrivatePath(dir, true)
	if err != nil || !privateDir {
		t.Fatalf("private directory policy = %v, %v", privateDir, err)
	}
	privateFile, err := IsPrivatePath(path, false)
	if err != nil || !privateFile {
		t.Fatalf("private file policy = %v, %v", privateFile, err)
	}
}

func TestWritePrivateFileExclusiveRefusesOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "artifact.txt")
	if err := WritePrivateFileExclusive(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFileExclusive(path, []byte("second")); err == nil {
		t.Fatal("exclusive private write overwrote an existing artifact")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first" {
		t.Fatalf("exclusive artifact = %q, %v", data, err)
	}
}

func TestConfineToRoot_WhenInsideRoot_ShouldResolve(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(sub, "file.txt")
	got, err := ConfineToRoot(root, candidate)
	if err != nil {
		t.Fatalf("ConfineToRoot inside root error = %v", err)
	}
	if got == "" {
		t.Fatal("ConfineToRoot inside root returned empty path")
	}
	// Resolved path should remain inside root and be absolute.
	if !filepath.IsAbs(got) {
		t.Fatalf("ConfineToRoot returned non-absolute path %q", got)
	}
	// The input may be a Windows short name; compare physical identities.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(resolvedRoot, got); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		t.Fatalf("ConfineToRoot returned path outside root: %q", got)
	}
}

func TestConfineToRoot_WhenRootItself_ShouldResolve(t *testing.T) {
	root := t.TempDir()
	got, err := ConfineToRoot(root, root)
	if err != nil {
		t.Fatalf("ConfineToRoot root itself error = %v", err)
	}
	if got == "" {
		t.Fatal("ConfineToRoot root itself returned empty path")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("ConfineToRoot root itself returned non-absolute %q", got)
	}
}

func TestConfineToRoot_WhenOutsideRoot_ShouldReject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	candidate := filepath.Join(outside, "file.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted a sibling directory outside root")
	}
}

func TestConfineToRoot_WhenParentTraversal_ShouldReject(t *testing.T) {
	root := t.TempDir()
	candidate := filepath.Join(root, "..", "evil.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted parent traversal outside root")
	}
}

func TestConfineToRoot_WhenSymlinkEscapes_ShouldReject(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "repo")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	candidate := filepath.Join(link, "secret.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted a symlink escape")
	}
}

func TestConfineToRoot_WhenEmptyArguments_ShouldError(t *testing.T) {
	root := t.TempDir()
	candidate := filepath.Join(root, "file.txt")
	if _, err := ConfineToRoot("", candidate); err == nil {
		t.Fatal("ConfineToRoot accepted empty root")
	}
	if _, err := ConfineToRoot("   ", candidate); err == nil {
		t.Fatal("ConfineToRoot accepted whitespace root")
	}
	if _, err := ConfineToRoot(root, ""); err == nil {
		t.Fatal("ConfineToRoot accepted empty candidate")
	}
	if _, err := ConfineToRoot(root, "   "); err == nil {
		t.Fatal("ConfineToRoot accepted whitespace candidate")
	}
}

func TestConfineToRoot_ShouldNotLeakOutsidePathInError(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	candidate := filepath.Join(outside, "secret.txt")
	_, err := ConfineToRoot(root, candidate)
	if err == nil {
		t.Fatal("ConfineToRoot accepted outside path")
	}
	if strings.Contains(err.Error(), outside) {
		t.Fatalf("error leaks outside path %q in %q", outside, err.Error())
	}
	if strings.Contains(err.Error(), candidate) {
		t.Fatalf("error leaks outside candidate %q in %q", candidate, err.Error())
	}
}

func TestProtectPrivateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "protect_me.txt")

	// Create a file with open permissions.
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ProtectPrivateFile(path); err != nil {
		t.Fatalf("ProtectPrivateFile failed: %v", err)
	}

	isPrivate, err := IsPrivatePath(path, false)
	if err != nil {
		t.Fatalf("IsPrivatePath failed: %v", err)
	}
	if !isPrivate {
		t.Fatal("File is not private after ProtectPrivateFile")
	}
}

func TestProtectPrivateFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.txt")

	if err := ProtectPrivateFile(path); err == nil {
		t.Fatal("ProtectPrivateFile on nonexistent file should return an error")
	}
}

func TestIsPrivatePath(t *testing.T) {
	t.Run("NonExistent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent")
		if _, err := IsPrivatePath(path, false); err == nil {
			t.Fatal("expected error for non-existent path")
		}
	})

	t.Run("PublicDirectory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "public")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Mock file permissions logic on Unix to avoid umask flakiness
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		isPrivate, err := IsPrivatePath(dir, true)
		if err != nil {
			t.Fatal(err)
		}
		if isPrivate {
			t.Fatal("expected directory to not be private")
		}
	})

	t.Run("PublicFile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "public.txt")
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Mock file permissions logic on Unix to avoid umask flakiness
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		isPrivate, err := IsPrivatePath(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if isPrivate {
			t.Fatal("expected file to not be private")
		}
	})

	t.Run("PrivateDirectory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "private")
		if err := EnsurePrivateDir(dir); err != nil {
			t.Fatal(err)
		}
		isPrivate, err := IsPrivatePath(dir, true)
		if err != nil {
			t.Fatal(err)
		}
		if !isPrivate {
			t.Fatal("expected directory to be private")
		}
	})

	t.Run("PrivateFile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "private.txt")
		if err := WritePrivateFile(path, []byte("data")); err != nil {
			t.Fatal(err)
		}
		isPrivate, err := IsPrivatePath(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if !isPrivate {
			t.Fatal("expected file to be private")
		}
	})
}
