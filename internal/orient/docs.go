package orient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/types"
)

// DocScan is every git-visible markdown, rst and txt document, the links
// between them, and (when an embedding engine is passed) the neighbour
// pairs the policy is allowed to treat as lineage evidence.
type DocScan struct {
	Facts           []types.Fact
	Bodies          map[string]string
	Unreadable      []string
	Empty           int
	Embedded        int
	CacheHits       int
	EmbeddingFailed int
	SimilarityNote  string
	SimilarityError string
	CacheWriteError string
}

// CollectDocs reads the work tree's tracked documents. Paths are
// repository-relative with forward slashes and no leading slash: a string
// that starts with '/' is stored as a name constant and joins nothing.
// An unreadable tracked document is listed on Unreadable and is not given
// a doc_file row; dropping it silently would hide it from the read set.
//
// emb may be nil. Similarity is then skipped and SimilarityNote says so.
// Failed documents are counted and named; successful vectors still supply
// the neighbour graph. No incomplete document centroid enters the cache.
func CollectDocs(ctx context.Context, root string, cfg config.OrientConfig, emb embedding.EmbeddingEngine) (*DocScan, error) {
	root, err := absPath(root)
	if err != nil {
		return nil, err
	}
	cfg = cfg.WithDefaults()
	if problems := cfg.Check("orient"); len(problems) > 0 {
		return nil, fmt.Errorf("%s: %s", problems[0].Path, problems[0].Message)
	}
	if err := ensureWorkTree(ctx, root); err != nil {
		return nil, err
	}
	paths, err := trackedDocs(ctx, root)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	scan := &DocScan{}
	known := make(map[string]struct{}, len(paths))
	bodies := make(map[string]string, len(paths))
	for _, p := range paths {
		known[p] = struct{}{}
	}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			scan.Unreadable = append(scan.Unreadable, p)
			continue
		}
		text := string(body)
		bodies[p] = text
		dir := path.Dir(p)
		if dir == "." {
			dir = ""
		}
		scan.Facts = append(scan.Facts, types.Fact{
			Predicate: "doc_file",
			Args:      []any{p, dir, int64(len(body)), int64(headingCount(text))},
		})
	}
	return collectDocBodies(ctx, root, cfg, emb, paths, bodies, scan)
}

func collectDocBodies(ctx context.Context, root string, cfg config.OrientConfig, emb embedding.EmbeddingEngine, paths []string, bodies map[string]string, scan *DocScan) (*DocScan, error) {
	cfg = cfg.WithDefaults()
	scan.Bodies = bodies
	readable := make(map[string]struct{}, len(bodies))
	for p := range bodies {
		readable[p] = struct{}{}
	}
	linkSeen := map[string]struct{}{}
	for _, p := range paths {
		text, ok := bodies[p]
		if !ok {
			continue
		}
		normalized := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
		digest := sha256.Sum256([]byte(normalized))
		scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_body_digest", Args: []any{p, hex.EncodeToString(digest[:])}})
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_subtree", Args: []any{p, dir}})
		}
		if path.Dir(p) == "." {
			scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_subtree", Args: []any{p, ""}})
		}
		for _, to := range linksIn(p, text, readable) {
			k := p + "\x00" + to
			if _, dup := linkSeen[k]; dup {
				continue
			}
			linkSeen[k] = struct{}{}
			scan.Facts = append(scan.Facts, types.Fact{
				Predicate: "doc_link",
				Args:      []any{p, to},
			})
		}
	}
	sort.Strings(scan.Unreadable)
	if emb == nil {
		for _, p := range paths {
			if body, ok := bodies[p]; ok && strings.TrimSpace(body) == "" {
				scan.Empty++
				scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_embedding_omitted", Args: []any{p, types.MangleAtom("/empty"), "no embeddable text"}})
			}
		}
		scan.SimilarityNote = "Similarity was not computed: no embedding engine was passed (an Ollama model has to be named; this command does not invent one). doc_similar and doc_cluster were not asserted."
		return scan, nil
	}
	embedded := embedDocs(ctx, root, cfg, emb, bodies)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vecs, hits := embedded.vectors, embedded.hits
	scan.CacheHits = hits
	scan.Empty = embedded.empty
	scan.Embedded = len(vecs)
	if embedded.cacheErr != nil {
		scan.CacheWriteError = embedded.cacheErr.Error()
	}
	for _, omission := range embedded.omitted {
		scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_embedding_omitted", Args: []any{omission.Path, types.MangleAtom(omission.Kind), omission.Detail}})
		if omission.Kind == "/error" {
			scan.EmbeddingFailed++
		}
	}
	for p := range vecs {
		status := types.MangleAtom("/embedded")
		if embedded.cached[p] {
			status = "/cache"
		}
		scan.Facts = append(scan.Facts, types.Fact{Predicate: "doc_embedding_status", Args: []any{p, status}})
	}
	if len(embedded.omitted) > 0 {
		var notice strings.Builder
		fmt.Fprintf(&notice, "orient: similarity retained %d documents; excluded %d (%d empty, %d embedding failures).\n", len(vecs), len(embedded.omitted), scan.Empty, scan.EmbeddingFailed)
		for _, omission := range embedded.omitted {
			fmt.Fprintf(&notice, "  %s %s: %s\n", omission.Path, omission.Kind, omission.Detail)
		}
		fmt.Fprint(os.Stderr, notice.String())
		if scan.EmbeddingFailed > 0 {
			scan.SimilarityError = notice.String()
		}
	}
	pairs := topKPairs(vecs, cfg.SimilarTopK, cfg.SimilarityFloorPermille)
	for _, pair := range pairs {
		scan.Facts = append(scan.Facts, types.Fact{
			Predicate: "doc_similar",
			Args:      []any{pair.a, pair.b, int64(pair.permille)},
		})
	}
	for path, id := range clusterIDs(pairs) {
		scan.Facts = append(scan.Facts, types.Fact{
			Predicate: "doc_cluster",
			Args:      []any{path, id},
		})
	}
	note := fmt.Sprintf(
		"doc_similar keeps at most %d neighbours of each document with cosine at or above %d permille. Pairs below the floor or outside that top-k are not facts; raise orient.similar_top_k or lower orient.similarity_floor_permille to widen the set. Embedded %d documents (%d from cache).",
		cfg.SimilarTopK, cfg.SimilarityFloorPermille, len(vecs), hits,
	)
	if len(embedded.omitted) > 0 {
		note += fmt.Sprintf(" Excluded %d documents (%d empty, %d embedding failures); all excluded paths and reasons are recorded in doc_embedding_omitted. The graph uses every successful document vector.", len(embedded.omitted), scan.Empty, scan.EmbeddingFailed)
	}
	if embedded.cacheErr != nil {
		note += fmt.Sprintf(" Vector cache write failed (%s); this run used the vectors it computed, and the next run will re-embed those documents.", embedded.cacheErr.Error())
	}
	scan.SimilarityNote = note
	return scan, nil
}

func trackedDocs(ctx context.Context, root string) ([]string, error) {
	cmd := gitCmd(ctx, root, "-c", "core.quotePath=false", "-c", "safe.directory=*", "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("orient docs: git ls-files: %w", err)
	}
	var paths []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		p := cleanGitPath(string(raw))
		if p == "" || !isDocPath(p) {
			continue
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// DocTieFacts assigns a unique lexicographic ordinal to each path. The
// read ranking and the cluster representative use it as the last key, so
// two documents that share a score and a commit day still cut the budget
// exactly, earlier path first. Call it once, over the whole set that can
// qualify (documents and instruction paths); a second ordinal space would
// make equal numbers mean different paths.
func DocTieFacts(paths []string) []types.Fact {
	seen := map[string]struct{}{}
	ordered := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	out := make([]types.Fact, 0, len(ordered))
	for i, p := range ordered {
		out = append(out, types.Fact{Predicate: "doc_tie", Args: []any{p, int64(i)}})
	}
	return out
}

// tiePaths are the paths Orient puts in one ordinal space: documents,
// cluster members, and instruction files.
func tiePaths(facts []types.Fact) []string {
	var out []string
	for _, f := range facts {
		switch f.Predicate {
		case "doc_file", "doc_cluster":
			if len(f.Args) > 0 {
				out = append(out, types.ExtractString(f.Args[0]))
			}
		case "agent_source":
			if len(f.Args) >= 5 && types.ExtractString(f.Args[2]) == "/instructions" {
				out = append(out, types.ExtractString(f.Args[4]))
			}
		}
	}
	return out
}

func headingCount(text string) int {
	n := 0
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isATXHeading(trim) {
			n++
		}
	}
	return n
}

func isATXHeading(trim string) bool {
	i := 0
	for i < len(trim) && trim[i] == '#' && i < 6 {
		i++
	}
	if i == 0 || i > 6 {
		return false
	}
	if i >= len(trim) {
		return false
	}
	if trim[i] != ' ' && trim[i] != '\t' {
		return false
	}
	return strings.TrimSpace(trim[i:]) != ""
}

var (
	reInline = regexp.MustCompile(`\[[^\]\n]*\]\(([^)\n]+)\)`)
	reRefUse = regexp.MustCompile(`\[[^\]\n]*\]\[([^\]\n]+)\]`)
	reRefDef = regexp.MustCompile(`(?m)^[ ]{0,3}\[([^\]\n]+)\]:[ \t]+<?([^>\s]+)>?`)
)

func linksIn(from, text string, known map[string]struct{}) []string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			b.WriteByte('\n')
			continue
		}
		if inFence {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	visible := b.String()
	defs := map[string]string{}
	for _, m := range reRefDef.FindAllStringSubmatch(visible, -1) {
		defs[strings.ToLower(strings.TrimSpace(m[1]))] = strings.TrimSpace(m[2])
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(raw string) {
		to, ok := resolveDocLink(from, raw, known)
		if !ok {
			return
		}
		if _, dup := seen[to]; dup {
			return
		}
		seen[to] = struct{}{}
		out = append(out, to)
	}
	for _, m := range reInline.FindAllStringSubmatch(visible, -1) {
		add(m[1])
	}
	for _, m := range reRefUse.FindAllStringSubmatch(visible, -1) {
		if dest, ok := defs[strings.ToLower(strings.TrimSpace(m[1]))]; ok {
			add(dest)
		}
	}
	for _, dest := range defs {
		add(dest)
	}
	sort.Strings(out)
	return out
}

func resolveDocLink(from, raw string, known map[string]struct{}) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if i := strings.IndexAny(raw, "#?"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "mailto:") {
		return "", false
	}
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return "", false
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	dir := path.Dir(from)
	if dir == "." {
		dir = ""
	}
	joined := path.Clean(path.Join(dir, raw))
	if joined == "." || joined == ".." || strings.HasPrefix(joined, "../") {
		return "", false
	}
	if _, ok := known[joined]; !ok {
		return "", false
	}
	if joined == from {
		return "", false
	}
	return joined, true
}

type simPair struct {
	a, b     string
	permille int
}

type neigh struct {
	j   int
	sim float64
}

// EmbeddingOmission records why one document has no vector.
type EmbeddingOmission struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type embeddingScan struct {
	vectors  map[string][]float32
	cached   map[string]bool
	hits     int
	empty    int
	omitted  []EmbeddingOmission
	cacheErr error
}

type embeddingChunk struct {
	path  string
	index int
	text  string
}

type embeddingResult struct {
	chunk embeddingChunk
	vec   []float32
	err   error
}

type documentEmbedding struct {
	hash string
	vecs [][]float32
	err  error
}

func embedDocs(ctx context.Context, root string, cfg config.OrientConfig, emb embedding.EmbeddingEngine, bodies map[string]string) embeddingScan {
	cfg = cfg.WithDefaults()
	scan := embeddingScan{vectors: map[string][]float32{}, cached: map[string]bool{}}
	paths := make([]string, 0, len(bodies))
	for p := range bodies {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	documents := map[string]*documentEmbedding{}
	var chunks []embeddingChunk
	for _, p := range paths {
		if ctx.Err() != nil {
			return scan
		}
		var parts []string
		for _, part := range chunkBytes(bodies[p], cfg.EmbeddingChunkBytes) {
			if strings.TrimSpace(part) != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			scan.empty++
			scan.omitted = append(scan.omitted, EmbeddingOmission{p, "/empty", "no embeddable text"})
			continue
		}
		sum := sha256.Sum256([]byte(bodies[p]))
		hash := hex.EncodeToString(sum[:])
		if vec, ok := readVectorCache(root, emb.Name(), hash, cfg.EmbeddingChunkBytes); ok && validVector(vec, emb.Dimensions()) == nil {
			scan.vectors[p], scan.cached[p] = vec, true
			scan.hits++
			continue
		}
		documents[p] = &documentEmbedding{hash: hash, vecs: make([][]float32, len(parts))}
		for i, part := range parts {
			chunks = append(chunks, embeddingChunk{p, i, part})
		}
	}

	jobs := make(chan []embeddingChunk, cfg.EmbeddingConcurrency)
	results := make(chan []embeddingResult, cfg.EmbeddingConcurrency)
	var workers sync.WaitGroup
	for i := 0; i < cfg.EmbeddingConcurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for batch := range jobs {
				if ctx.Err() != nil {
					return
				}
				rows := embedBatch(ctx, emb, batch, cfg.EmbeddingRetryAttempts)
				select {
				case results <- rows:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for start := 0; start < len(chunks); start += cfg.EmbeddingBatchSize {
			end := start + cfg.EmbeddingBatchSize
			if end > len(chunks) {
				end = len(chunks)
			}
			select {
			case jobs <- chunks[start:end]:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	for batch := range results {
		for _, row := range batch {
			doc := documents[row.chunk.path]
			if row.err != nil {
				if doc.err == nil {
					doc.err = fmt.Errorf("chunk %d: %w", row.chunk.index+1, row.err)
				}
				continue
			}
			doc.vecs[row.chunk.index] = row.vec
		}
	}
	if ctx.Err() != nil {
		return scan
	}
	var cacheErrors []error
	for _, p := range paths {
		if ctx.Err() != nil {
			return scan
		}
		doc := documents[p]
		if doc == nil {
			continue
		}
		dims := 0
		for _, vec := range doc.vecs {
			if doc.err != nil {
				break
			}
			if err := validVector(vec, dims); err != nil {
				doc.err = err
				break
			}
			dims = len(vec)
		}
		if doc.err == nil {
			vec := centroid(doc.vecs)
			if err := validVector(vec, dims); err != nil {
				doc.err = err
			} else {
				scan.vectors[p] = vec
				if err := writeVectorCache(root, emb.Name(), doc.hash, cfg.EmbeddingChunkBytes, vec); err != nil {
					cacheErrors = append(cacheErrors, fmt.Errorf("%s: %w", p, err))
				}
			}
		}
		if doc.err != nil {
			scan.omitted = append(scan.omitted, EmbeddingOmission{p, "/error", doc.err.Error()})
		}
	}
	sort.Slice(scan.omitted, func(i, j int) bool { return scan.omitted[i].Path < scan.omitted[j].Path })
	scan.cacheErr = errors.Join(cacheErrors...)
	return scan
}

// An exhausted batch is bisected so one rejected item cannot discard its peers.
// Retries and splits stay inside a worker; request concurrency cannot multiply.
func embedBatch(ctx context.Context, emb embedding.EmbeddingEngine, batch []embeddingChunk, attempts int) []embeddingResult {
	texts := make([]string, len(batch))
	for i, chunk := range batch {
		texts[i] = chunk.text
	}
	var vecs [][]float32
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
		vecs, err = emb.EmbedBatch(ctx, texts)
		if err == nil && len(vecs) != len(batch) {
			err = fmt.Errorf("embedding batch returned %d vectors for %d chunks", len(vecs), len(batch))
		}
		if err == nil {
			for i, vec := range vecs {
				if problem := validVector(vec, emb.Dimensions()); problem != nil {
					err = fmt.Errorf("embedding batch item %d: %w", i+1, problem)
					break
				}
			}
		}
		if err == nil {
			out := make([]embeddingResult, len(batch))
			for i, chunk := range batch {
				out[i] = embeddingResult{chunk: chunk, vec: vecs[i]}
			}
			return out
		}
	}
	if len(batch) > 1 && ctx.Err() == nil {
		middle := len(batch) / 2
		left := embedBatch(ctx, emb, batch[:middle], attempts)
		return append(left, embedBatch(ctx, emb, batch[middle:], attempts)...)
	}
	out := make([]embeddingResult, len(batch))
	for i, chunk := range batch {
		out[i] = embeddingResult{chunk: chunk, err: fmt.Errorf("embedding request failed after at most %d attempts: %w", attempts, err)}
	}
	return out
}

func validVector(vec []float32, dims int) error {
	if len(vec) == 0 {
		return fmt.Errorf("empty embedding")
	}
	if dims > 0 && len(vec) != dims {
		return fmt.Errorf("embedding length %d, want %d", len(vec), dims)
	}
	nonzero := false
	for _, x := range vec {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return fmt.Errorf("non-finite embedding")
		}
		nonzero = nonzero || x != 0
	}
	if !nonzero {
		return fmt.Errorf("zero embedding")
	}
	return nil
}

func chunkBytes(s string, n int) []string {
	if n <= 0 {
		return nil
	}
	b := []byte(s)
	if len(b) == 0 {
		return nil
	}
	var out []string
	for len(b) > 0 {
		if len(b) <= n {
			out = append(out, string(b))
			break
		}
		cut := n
		for cut > 0 && !utf8.RuneStart(b[cut]) {
			cut--
		}
		if cut == 0 {
			cut = n
		}
		out = append(out, string(b[:cut]))
		b = b[cut:]
	}
	return out
}

func centroid(vs [][]float32) []float32 {
	dims := len(vs[0])
	sum := make([]float64, dims)
	for _, v := range vs {
		for i, x := range v {
			sum[i] += float64(x)
		}
	}
	out := make([]float32, dims)
	n := float64(len(vs))
	for i := range out {
		out[i] = float32(sum[i] / n)
	}
	return out
}

func cosine(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func topKPairs(vecs map[string][]float32, k, floor int) []simPair {
	if k < 1 || len(vecs) < 2 {
		return nil
	}
	paths := make([]string, 0, len(vecs))
	for p := range vecs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	chosen := map[[2]string]int{}
	for i := range paths {
		best := make([]neigh, 0, k)
		for j := range paths {
			if i == j {
				continue
			}
			sim := cosine(vecs[paths[i]], vecs[paths[j]])
			pm := int(math.Round(sim * 1000))
			if pm < floor {
				continue
			}
			best = insertNeigh(best, neigh{j, sim}, k, paths)
		}
		for _, nb := range best {
			a, b := paths[i], paths[nb.j]
			if a > b {
				a, b = b, a
			}
			pm := int(math.Round(nb.sim * 1000))
			key := [2]string{a, b}
			if prev, ok := chosen[key]; !ok || pm > prev {
				chosen[key] = pm
			}
		}
	}
	out := make([]simPair, 0, len(chosen))
	for key, pm := range chosen {
		out = append(out, simPair{a: key[0], b: key[1], permille: pm})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].a != out[j].a {
			return out[i].a < out[j].a
		}
		return out[i].b < out[j].b
	})
	return out
}

func insertNeigh(best []neigh, n neigh, k int, paths []string) []neigh {
	better := func(a, b neigh) bool {
		if a.sim != b.sim {
			return a.sim > b.sim
		}
		return paths[a.j] < paths[b.j]
	}
	best = append(best, n)
	sort.Slice(best, func(i, j int) bool { return better(best[i], best[j]) })
	if len(best) > k {
		best = best[:k]
	}
	return best
}

// clusterIDs is a union-find whose root is the lexicographic minimum path
// in the component. Only paths that appear in a pair are members; an
// isolate is not a cluster, so it cannot be elected a representative of one.
func clusterIDs(pairs []simPair) map[string]string {
	parent := map[string]string{}
	var find func(string) string
	find = func(p string) string {
		if _, ok := parent[p]; !ok {
			parent[p] = p
		}
		if parent[p] != p {
			parent[p] = find(parent[p])
		}
		return parent[p]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}
	for _, pair := range pairs {
		union(pair.a, pair.b)
	}
	out := make(map[string]string, len(parent))
	for p := range parent {
		out[p] = find(p)
	}
	return out
}

var cacheMu sync.Mutex

func cacheFile(root, engineName, hash string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', ' ':
			return '_'
		default:
			return r
		}
	}, engineName)
	if safe == "" {
		safe = "embedder"
	}
	// Under .nerd/cache, which init ignores: embeddings are regenerable and a
	// committed .nerd must not carry them.
	return filepath.Join(root, ".nerd", "cache", "orient", "vectors", safe, hash)
}

func readVectorCache(root, engineName, hash string, chunk int) ([]float32, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	b, err := os.ReadFile(cacheFile(root, engineName, hash))
	if err != nil || len(b) < 14 || string(b[:6]) != "ORVEC1" {
		return nil, false
	}
	gotChunk := binary.LittleEndian.Uint32(b[6:10])
	dims := binary.LittleEndian.Uint32(b[10:14])
	if int(gotChunk) != chunk || dims == 0 {
		return nil, false
	}
	need := 14 + int(dims)*4
	if len(b) != need {
		return nil, false
	}
	out := make([]float32, dims)
	for i := range out {
		bits := binary.LittleEndian.Uint32(b[14+i*4:])
		out[i] = math.Float32frombits(bits)
	}
	return out, true
}

func writeVectorCache(root, engineName, hash string, chunk int, vec []float32) error {
	if len(vec) == 0 {
		return nil
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	dest := cacheFile(root, engineName, hash)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("ORVEC1")
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(chunk))
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(vec)))
	buf.Write(hdr[:])
	for _, v := range vec {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
		buf.Write(b[:])
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
