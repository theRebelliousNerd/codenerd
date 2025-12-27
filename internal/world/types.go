package world

import "codenerd/internal/types"

// Type aliases to break import cycle between world and core.
// The canonical definitions are in the types package.

// Fact is an alias to types.Fact for use within the world package.
type Fact = types.Fact

// MangleAtom is an alias to types.MangleAtom for use within the world package.
type MangleAtom = types.MangleAtom
