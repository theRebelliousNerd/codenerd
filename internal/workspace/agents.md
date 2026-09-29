# Workspace membership

- This leaf package owns membership; walkers prune rejected directories before descent and keep only their purpose filters.
- Git reads use `GIT_OPTIONAL_LOCKS=0`. Preserve NUL-delimited path bytes, gitlink boundaries, and root alias identity.
- `For` reads target workspace ignore patterns; `Open` uses explicit configuration. Defaults belong only to `config.DefaultWorldConfig`.
- Serialize dynamic ignore checks with refresh. Explicit refresh must see nested ignore edits and new files.
- Test git, fallback, dynamic paths, directory-read pruning, and concurrent refresh. For repo-contained test temp directories, set `GIT_CEILING_DIRECTORIES` to the temp parent so fallback fixtures cannot discover the enclosing repository.
- Semantic treatment is still an orientation integration gap; do not invent a Go decision policy for it.
