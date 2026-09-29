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
	Unreadable      []string
	Empty           int
	Embedded        int
	CacheHits       int
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
// The first embedding error discards every doc_similar and doc_cluster
// row: a partial neighbour graph would elect representatives from
// whichever documents happened to embed. Vectors already cached stay
// cached.
func CollectDocs(ctx context.Context, root string, cfg config.OrientConfig, emb embedding.EmbeddingEngine) (*DocScan, error) {
	root, err := absPath(root)
	if err != nil {
		return nil, err
	}
	cfg = cfg.WithDefaults()
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
		scan.SimilarityNote = "Similarity was not computed: no embedding engine was passed (an Ollama model has to be named; this command does not invent one). doc_similar and doc_cluster were not asserted."
		return scan, nil
	}
	vecs, hits, empty, cacheErr, embedErr := embedDocs(ctx, root, cfg.EmbeddingChunkBytes, emb, bodies)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	scan.CacheHits = hits
	scan.Empty = empty
	scan.Embedded = len(vecs)
	if cacheErr != nil {
		scan.CacheWriteError = cacheErr.Error()
	}
	if embedErr != nil {
		scan.SimilarityError = embedErr.Error()
		scan.SimilarityNote = fmt.Sprintf(
			"Similarity was not computed: %s. No doc_similar or doc_cluster facts were asserted; a partial neighbour graph would pick representatives from whichever documents happened to embed.",
			embedErr.Error(),
		)
		return scan, nil
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
	if empty > 0 {
		note += fmt.Sprintf(" %d empty documents were not embedded and have no neighbours.", empty)
	}
	if cacheErr != nil {
		note += fmt.Sprintf(" Vector cache write failed (%s); this run used the vectors it computed, and the next run will re-embed those documents.", cacheErr.Error())
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

func embedDocs(ctx context.Context, root string, chunk int, emb embedding.EmbeddingEngine, bodies map[string]string) (map[string][]float32, int, int, error, error) {
	paths := make([]string, 0, len(bodies))
	empty := 0
	for p, body := range bodies {
		if strings.TrimSpace(body) == "" {
			empty++
			continue
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	parent := ctx
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	type result struct {
		path  string
		vec   []float32
		hit   bool
		err   error
		cache error
	}
	out := make(chan result, len(paths))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, p := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out <- result{path: p, err: ctx.Err()}
				return
			}
			vec, hit, cacheErr, err := embedOne(ctx, root, emb, p, bodies[p], chunk)
			out <- result{path: p, vec: vec, hit: hit, err: err, cache: cacheErr}
		}(p)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	vecs := make(map[string][]float32, len(paths))
	hits := 0
	var first error
	var cacheErr error
	for r := range out {
		if r.cache != nil && cacheErr == nil {
			cacheErr = r.cache
		}
		if r.err != nil {
			if parent.Err() != nil {
				continue
			}
			if first == nil && !errors.Is(r.err, context.Canceled) {
				first = fmt.Errorf("%s: %w", r.path, r.err)
				cancel()
			}
			continue
		}
		if len(r.vec) == 0 {
			continue
		}
		vecs[r.path] = r.vec
		if r.hit {
			hits++
		}
	}
	if parent.Err() != nil {
		return nil, 0, empty, cacheErr, parent.Err()
	}
	if first != nil {
		return nil, hits, empty, cacheErr, first
	}
	return vecs, hits, empty, cacheErr, nil
}

func embedOne(ctx context.Context, root string, emb embedding.EmbeddingEngine, p, body string, chunk int) ([]float32, bool, error, error) {
	sum := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(sum[:])
	if vec, ok := readVectorCache(root, emb.Name(), hash, chunk); ok {
		return vec, true, nil, nil
	}
	parts := chunkBytes(body, chunk)
	vecs := make([][]float32, 0, len(parts))
	var dims int
	for _, part := range parts {
		v, err := emb.Embed(ctx, part)
		if err != nil {
			return nil, false, nil, err
		}
		if len(v) == 0 {
			return nil, false, nil, fmt.Errorf("empty embedding")
		}
		if dims == 0 {
			dims = len(v)
		} else if len(v) != dims {
			return nil, false, nil, fmt.Errorf("embedding length %d, want %d", len(v), dims)
		}
		vecs = append(vecs, v)
	}
	cent := centroid(vecs)
	err := writeVectorCache(root, emb.Name(), hash, chunk, cent)
	return cent, false, err, nil
}

func chunkBytes(s string, n int) []string {
	if n <= 0 {
		n = 2000
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
	return filepath.Join(root, ".nerd", "orient", "vectors", safe, hash)
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
