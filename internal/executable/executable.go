// Package executable recognizes compiled executables by their file header.
//
// The committed-binary audit and the recurse ratchet share this one
// definition: an executable is never source, so the audit fails a tracked one
// and the ratchet never commits one an attempt built.
package executable

import (
	"bytes"
	"io"
	"os"
)

// magic lists the headers of the executable formats codeNERD builds or meets.
var magic = []struct {
	name  string
	magic []byte
}{
	{"ELF (Linux)", []byte{0x7F, 'E', 'L', 'F'}},
	{"PE (Windows)", []byte{'M', 'Z'}},
	{"Mach-O 64 (macOS)", []byte{0xCF, 0xFA, 0xED, 0xFE}},
	{"Mach-O 32 (macOS)", []byte{0xCE, 0xFA, 0xED, 0xFE}},
	{"Mach-O universal (macOS)", []byte{0xCA, 0xFE, 0xBA, 0xBE}},
}

// Kind reports the executable format of the file at path, or false when it
// is not one. A file that will not open is not an executable for this
// purpose: a sparse checkout or a dangling symlink is nobody's binary.
func Kind(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	header := make([]byte, 4)
	n, _ := io.ReadFull(f, header)
	return KindOf(header[:n])
}

// KindOf reports the executable format a file starting with header has.
func KindOf(header []byte) (string, bool) {
	for _, m := range magic {
		if bytes.HasPrefix(header, m.magic) {
			return m.name, true
		}
	}
	return "", false
}
