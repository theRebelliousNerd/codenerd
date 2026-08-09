//go:build !windows

package security

import "os"

func protectPrivatePath(path string, directory bool) error {
	mode := os.FileMode(0o600)
	if directory {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}

func isPrivatePath(path string, directory bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	want := os.FileMode(0o600)
	if directory {
		want = 0o700
	}
	return info.Mode().Perm() == want, nil
}
