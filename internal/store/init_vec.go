package store

import (
	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	// Register the sqlite-vec extension with the mattn/go-sqlite3 driver.
	// vec.Auto() registers it as an auto-loadable extension.
	vec.Auto()
}
