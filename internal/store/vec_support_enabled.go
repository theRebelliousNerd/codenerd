//go:build sqlite_vec && cgo

package store

// When built with sqlite_vec, require the extension to be present.
const defaultRequireVec = true

