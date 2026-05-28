package factsnap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCodecParity proves the cross-codec contract: gzip and zstd produce
// byte-different files on disk (different compression algorithms) but
// decode to the SAME semantic fact set. This is what callers rely on
// when picking a codec by environment (gzip for portability, zstd for
// size) — the choice must not change observable contents.
//
// Property-tested across three sizes (100, 1k, 10k) so the comparison
// exercises the small-batch, mid-batch, and beyond-typical-cap paths
// of the SimpleColumn writer.
func TestCodecParity(t *testing.T) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		n := n
		t.Run(sizeName(n), func(t *testing.T) {
			facts := sampleFacts(n)
			dir := t.TempDir()
			gzPath := filepath.Join(dir, "parity")
			zstPath := filepath.Join(dir, "parity")

			require.NoError(t, WriteCodec(gzPath, facts, CodecGzip), "gzip write")
			require.NoError(t, WriteCodec(zstPath, facts, CodecZstd), "zstd write")

			gzCanonical := CanonicalPath(gzPath, CodecGzip)
			zstCanonical := CanonicalPath(zstPath, CodecZstd)

			// Byte-different: same input, different container framing.
			gzBytes := mustReadAll(t, gzCanonical)
			zstBytes := mustReadAll(t, zstCanonical)
			require.NotEqual(t, gzBytes, zstBytes,
				"gz and zst should never produce identical byte streams")

			// Magic header sanity: gzip starts 0x1f 0x8b; zstd starts 0x28 0xb5 0x2f 0xfd.
			require.GreaterOrEqual(t, len(gzBytes), 2, "gz file too small")
			require.Equal(t, byte(0x1f), gzBytes[0], "gzip magic byte 0")
			require.Equal(t, byte(0x8b), gzBytes[1], "gzip magic byte 1")
			require.GreaterOrEqual(t, len(zstBytes), 4, "zst file too small")
			require.Equal(t, byte(0x28), zstBytes[0], "zstd magic byte 0")
			require.Equal(t, byte(0xb5), zstBytes[1], "zstd magic byte 1")

			// Semantic identity: decode both, normalise, compare.
			gotGz, err := Read(gzCanonical)
			require.NoError(t, err, "read gzip")
			gotZst, err := Read(zstCanonical)
			require.NoError(t, err, "read zstd")

			require.Len(t, gotGz, n, "gz fact count")
			require.Len(t, gotZst, n, "zst fact count")

			// equalishFacts (defined in factsnap_test.go) normalises int↔int64
			// and string↔MangleAtom so the comparison is robust.
			require.True(t, equalishFacts(gotGz, gotZst),
				"gz and zst must decode to identical fact set (size=%d)", n)

			// And both must equal the original.
			require.True(t, equalishFacts(facts, gotGz),
				"gz read must equal original facts (size=%d)", n)
			require.True(t, equalishFacts(facts, gotZst),
				"zst read must equal original facts (size=%d)", n)
		})
	}
}

// TestCodecParity_PathRewriting verifies CanonicalPath emits the right
// suffix per codec. This is a contract test for callers that pass a
// logical name (no extension) and rely on the package to do the right
// thing — historically a place where path-mangling bugs sneak in.
func TestCodecParity_PathRewriting(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		codec Codec
		want  string
	}{
		{"plain_gzip", "snap", CodecGzip, "snap" + ExtGzip},
		{"plain_zstd", "snap", CodecZstd, "snap" + ExtZstd},
		{"existing_gzip_kept", "snap" + ExtGzip, CodecGzip, "snap" + ExtGzip},
		{"existing_zstd_kept", "snap" + ExtZstd, CodecZstd, "snap" + ExtZstd},
		{"json_replaced_gzip", "snap" + ExtJSON, CodecGzip, "snap" + ExtGzip},
		{"json_replaced_zstd", "snap" + ExtJSON, CodecZstd, "snap" + ExtZstd},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := CanonicalPath(tc.in, tc.codec)
			require.Equal(t, tc.want, got)
		})
	}
}

func mustReadAll(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	require.NotEmpty(t, data, "file is empty: %s", path)
	return data
}

func sizeName(n int) string {
	switch n {
	case 100:
		return "100_facts"
	case 1000:
		return "1k_facts"
	case 10000:
		return "10k_facts"
	default:
		return "n_facts"
	}
}
