//go:build !windows

package config

import "os"

func replacePrivateFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
