//go:build sqlite_vec && cgo

package store

import (
	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	// Register the sqlite-vec extension with the mattn/go-sqlite3 driver.
	// vec.Auto() registers it as an auto-loadable extension.
	vec.Auto()
}
