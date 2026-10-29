package orient

import (
	"fmt"
	"path/filepath"
	"strings"
)

func absPath(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("orient: workspace is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return abs, nil
}
