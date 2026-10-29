// Package workspace is the one authority for "is this path part of the
// workspace?" Every production walker asks Membership instead of keeping a
// private ignore list.
//
// In a git work tree the member file set is one
// `git ls-files -z -co --exclude-standard` (GIT_OPTIONAL_LOCKS=0). Member
// directories are the ancestors of those files. A path created after that
// snapshot is decided by a batched `git check-ignore -z --stdin` and cached.
// The snapshot is taken again when .git/index, HEAD, the ref HEAD names, the
// root .gitignore, or info/exclude changes, and when Refresh is called (scan
// ticks call it). A nested .gitignore edit is visible to check-ignore
// immediately for paths that were not in the snapshot; paths already in the
// snapshot stay until the next refresh.
//
// Gitlinks (mode 160000) and nested repositories are boundaries. Git lists a
// nested repo as a single trailing-slash path and does not list the files
// inside it; check-ignore would still call those files unignored, so the
// boundary is closed from the listing itself. Includes is true for the
// boundary path (git reported it) unless a user pattern or an always-excluded
// segment drops it. IncludesDir is false, so a walk never opens it, and
// nothing under it is a member. The boundary is not a file in Files.
//
// Outside a work tree, or when the git binary is missing, membership is only
// the user patterns below plus the always-excluded directories. A .git that
// is not a real repository (for example a test fixture that only has HEAD)
// is that fallback, not an error. A confirmed work tree whose ls-files then
// fails is an error: guessing would walk the trees git is supposed to hide.
//
// world.ignore_patterns is applied on top in both modes, so a tracked tree
// the user named is still dropped. Git already applied .gitignore; a leading
// ! here re-includes only an earlier user pattern, never a gitignored path
// and never .git or .nerd. The only directories that are always excluded,
// in every mode, are .git and .nerd. Hidden directories are ordinary members
// when git (or the user patterns, outside git) says so.
//
// Pattern language, last match wins:
//
//   - Separators are `/`. `\` is treated as `/`.
//   - A leading `!` negates. `!` only overrides earlier user patterns.
//   - A trailing `/` is directory-only and is stripped before the anchor
//     test, so `build/` matches a directory named build anywhere, while
//     `/build/` and `src/build/` are relative to the workspace root.
//   - A pattern that still contains no `/` matches that name in any directory.
//   - A pattern that contains `/` is workspace-root-relative. One leading `/`
//     is stripped. `*` and `?` match inside one segment. `**` is special only
//     as its own segment, where it matches zero or more segments.
//   - A path matches when the path itself matches or any ancestor directory
//     matches as a directory. The last matching pattern wins.
//   - An unclosed `[` is not a pattern: it never matches. WorldConfig.Check
//     reports it.
//
// On Windows the user-pattern comparison is case-folded, because git's
// core.ignorecase defaults to true there. Elsewhere it is case-sensitive.
// Git's own snapshot is not refolded; git already decided those paths.
//
// A directory that user patterns exclude is not opened when no later
// negation could match something inside it. An unanchored negation can match
// in any directory, so those directories stay open. Entering an extra
// directory is safe; skipping a file a negation re-includes is not.
//
// For reads the workspace's .nerd/config.json only far enough to decode
// world.ignore_patterns. An absent file, an absent key, an empty array, and
// invalid JSON all use config.DefaultWorldConfig's list (the same rule as
// GetWorldConfig). The file is not logged. Open with a non-nil slice does
// not read the file: a nil slice means For, and an empty slice means no
// extra patterns.
package workspace
