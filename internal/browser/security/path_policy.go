package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
)

// PathPolicy confines model-supplied browser artifacts to explicit roots.
type PathPolicy struct {
	baseDir string
	roots   []string
}

// NewPathPolicy resolves allowed roots against baseDir.
func NewPathPolicy(baseDir string, roots []string) (*PathPolicy, error) {
	if strings.TrimSpace(baseDir) == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get path policy base directory: %w", err)
		}
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path policy base directory: %w", err)
	}
	if len(roots) == 0 {
		// Default writable roots must cover every directory the browser subsystem writes to;
		// snapshots was missing which made `nerd browser snapshot` fail at the final write.
		roots = []string{filepath.Join(".nerd", "browser", "screenshots"), filepath.Join(".nerd", "browser", "traces"), filepath.Join(".nerd", "browser", "snapshots")}
	}
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(baseDir, root)
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve writable root %q: %w", root, err)
		}
		resolved = append(resolved, filepath.Clean(absolute))
	}
	if len(resolved) == 0 {
		return nil, errors.New("browser writable_roots must contain at least one path")
	}
	return &PathPolicy{baseDir: filepath.Clean(baseDir), roots: resolved}, nil
}

// DefaultRoot returns the first configured writable root.
func (p *PathPolicy) DefaultRoot() string {
	if p == nil || len(p.roots) == 0 {
		return ""
	}
	return p.roots[0]
}

// ResolveForWrite validates the final path, including existing symlink parents.
func (p *PathPolicy) ResolveForWrite(requested, defaultRoot, defaultName string) (string, error) {
	if p == nil {
		return "", errors.New("browser write path policy is not configured")
	}
	if strings.TrimSpace(requested) == "" {
		requested = defaultName
	}
	if !filepath.IsAbs(requested) {
		base := p.baseDir
		if strings.TrimSpace(defaultRoot) != "" {
			base = defaultRoot
			if !filepath.IsAbs(base) {
				base = filepath.Join(p.baseDir, base)
			}
		}
		requested = filepath.Join(base, requested)
	}
	target, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve browser output path: %w", err)
	}
	target = filepath.Clean(target)
	resolvedTarget, err := resolveExistingPrefix(target)
	if err != nil {
		return "", err
	}
	for _, root := range p.roots {
		resolvedRoot, err := resolveExistingPrefix(root)
		if err != nil {
			return "", err
		}
		if pathWithin(resolvedRoot, resolvedTarget) {
			return target, nil
		}
	}
	return "", fmt.Errorf("browser output path %q is outside writable_roots", target)
}

// Upload confinement is adapted from BrowserNERD's Apache-2.0 browser-act
// contract. A page receives only regular workspace files; the shared secret
// matcher also applies to both the requested name and its resolved target.

// ResolveForUpload validates an existing workspace file for a page's file input.
func (p *PathPolicy) ResolveForUpload(requested string) (string, error) {
	if p == nil || p.baseDir == "" {
		return "", errors.New("browser upload path policy is not configured")
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", errors.New("browser upload path is empty")
	}
	for _, part := range strings.Split(strings.ReplaceAll(requested, `\`, "/"), "/") {
		if part == ".." {
			return "", errors.New("browser upload path contains parent traversal")
		}
	}
	if tools.IsSecretPath(requested) {
		return "", errors.New("browser upload path names a secret file")
	}
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(p.baseDir, requested)
	}
	target, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve browser upload path: %w", err)
	}
	base, err := filepath.EvalSymlinks(p.baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve browser upload workspace: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve browser upload file: %w", err)
	}
	if !pathWithin(base, resolved) {
		return "", errors.New("browser upload file is outside the workspace root")
	}
	if tools.IsSecretPath(target) || tools.IsSecretPath(resolved) {
		return "", errors.New("browser upload path names a secret file")
	}
	relative, err := filepath.Rel(base, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve browser upload file under workspace: %w", err)
	}
	if err := validateUploadRelativePath(relative); err != nil {
		return "", err
	}
	// Check the alias as well so a dot path cannot disguise an ordinary file.
	// Different spellings of the workspace itself still share its resolved root.
	for _, root := range []string{p.baseDir, base} {
		if !pathWithin(root, target) {
			continue
		}
		relative, err := filepath.Rel(root, target)
		if err != nil {
			return "", fmt.Errorf("resolve browser upload alias under workspace: %w", err)
		}
		if err := validateUploadRelativePath(relative); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect browser upload file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("browser upload path is not a regular file")
	}
	return resolved, nil
}

func validateUploadRelativePath(relative string) error {
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return errors.New("browser upload path contains a dot path")
		}
		// A Windows alternate data stream can evade filename-based secret checks.
		if strings.Contains(part, ":") {
			return errors.New("browser upload path contains an alternate data stream")
		}
	}
	return nil
}

// ConfineToRoot resolves candidate and reports the resolved absolute path
// only when it lies inside root. It is the read-side counterpart to
// ResolveForWrite: repository tracing must never read a file outside the
// root the operator named, and a symlink is the ordinary way that happens
// by accident.
func ConfineToRoot(root, candidate string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("root must not be empty")
	}
	if strings.TrimSpace(candidate) == "" {
		return "", errors.New("candidate must not be empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve candidate: %w", err)
	}
	absCandidate = filepath.Clean(absCandidate)
	resolvedRoot, err := resolveExistingPrefix(absRoot)
	if err != nil {
		return "", err
	}
	resolvedCandidate, err := resolveExistingPrefix(absCandidate)
	if err != nil {
		return "", err
	}
	if pathWithin(resolvedRoot, resolvedCandidate) {
		return resolvedCandidate, nil
	}
	return "", errors.New("path escapes root")
}

// EnsurePrivateDir creates an owner-only directory where supported.
func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return protectPrivatePath(path, true)
}

// WritePrivateFile writes an owner-only browser artifact.
func WritePrivateFile(path string, data []byte) error {
	return writePrivateFile(path, data, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, false)
}

// WritePrivateFileExclusive creates an owner-only artifact without overwriting
// an existing path.
func WritePrivateFileExclusive(path string, data []byte) error {
	return writePrivateFile(path, data, os.O_CREATE|os.O_EXCL|os.O_WRONLY, true)
}

func writePrivateFile(path string, data []byte, flags int, removeOnFailure bool) error {
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	if err := file.Close(); err != nil {
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	if err := ProtectPrivateFile(path); err != nil {
		if removeOnFailure {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}

// ProtectPrivateFile applies the platform's current-user-only file policy.
func ProtectPrivateFile(path string) error {
	return protectPrivatePath(path, false)
}

// IsPrivatePath verifies the platform's current-user-only path policy.
func IsPrivatePath(path string, directory bool) (bool, error) {
	return isPrivatePath(path, directory)
}

// pathWithin reports whether target lies inside root, including root itself.
// It uses filepath.Rel so a trailing separator does not change the result:
// filepath.Clean normalises "/a/b" and "/a/b/" to the same path, and Rel
// then yields "." for equality rather than requiring an exact string match.
func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func resolveExistingPrefix(path string) (string, error) {
	current := filepath.Clean(path)
	missing := make([]string, 0, 4)
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve browser output symlink %q: %w", current, err)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect browser output path %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
