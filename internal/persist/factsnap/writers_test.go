package factsnap

import "codenerd/internal/types"

// The convenience writers the tests use. Production writes through WritePath
// (internal/persist/snapshot.Export); these spellings had no production caller.

// Write serialises facts to path using SimpleColumn + gzip.
func Write(path string, facts []types.Fact) error {
	return WriteCodec(path, facts, CodecGzip)
}

// WriteCodec serialises facts to path using the requested codec.
func WriteCodec(path string, facts []types.Fact, codec Codec) error {
	return WriteOptions(path, facts, Options{Codec: codec})
}

// WriteOptions serialises facts to path under opts.
func WriteOptions(path string, facts []types.Fact, opts Options) error {
	_, err := WritePath(path, facts, opts)
	return err
}

// CanonicalPath returns path rewritten with the canonical extension for codec.
func CanonicalPath(path string, codec Codec) string {
	return ensureExt(path, codec)
}
