//go:build !sqlite_vec || !cgo

package store

// Default builds treat sqlite-vec as optional.
const defaultRequireVec = false

