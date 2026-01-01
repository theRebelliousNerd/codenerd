package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

const (
	reflectionWorkerInterval = 45 * time.Second
	reflectionBatchSize      = 32
)

// SetReflectionConfig configures reflection embedding for the LocalStore.
func (s *LocalStore) SetReflectionConfig(cfg config.ReflectionConfig) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.reflectionCfg = &cfg
	enabled := cfg.Enabled
	running := s.reflectionStop != nil
	s.mu.Unlock()

	if enabled {
		s.startReflectionWorker()
		return
	}
	if running {
		s.stopReflectionWorker()
	}
}

func (s *LocalStore) startReflectionWorker() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.reflectionStop != nil {
		s.mu.Unlock()
		return
	}
	if s.embeddingEngine == nil || s.reflectionCfg == nil || !s.reflectionCfg.Enabled {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	s.reflectionStop = stop
	s.reflectionDone = done
	s.mu.Unlock()

	go s.runReflectionWorker(stop, done)
}

func (s *LocalStore) stopReflectionWorker() {
	if s == nil {
		return
	}
	s.mu.Lock()
	stop := s.reflectionStop
	done := s.reflectionDone
	s.reflectionStop = nil
	s.reflectionDone = nil
	s.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *LocalStore) runReflectionWorker(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(reflectionWorkerInterval)
	defer ticker.Stop()

	s.processReflectionCycle()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.processReflectionCycle()
		}
	}
}

func (s *LocalStore) processReflectionCycle() {
	if s == nil || s.traceStore == nil {
		return
	}

	s.mu.RLock()
	cfg := s.reflectionCfg
	engine := s.embeddingEngine
	s.mu.RUnlock()

	if cfg == nil || !cfg.Enabled || engine == nil {
		return
	}

	expectedTask := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
	expectedModel := engine.Name()
	expectedDim := engine.Dimensions()

	backlog := 0
	if count, err := s.traceStore.CountTraceEmbeddingBacklog(expectedModel, expectedDim, expectedTask); err == nil {
		backlog = count
	}
	skipSuccess := cfg.BacklogWatermark > 0 && backlog > cfg.BacklogWatermark

	batchSize := reflectionBatchSize
	if skipSuccess && batchSize > 16 {
		batchSize = 16
	}

	candidates, err := s.traceStore.ListTraceEmbeddingCandidates(batchSize, skipSuccess, expectedModel, expectedDim, expectedTask)
	if err != nil || len(candidates) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	updates, embedCount, err := s.buildTraceEmbeddingUpdates(ctx, candidates, engine, expectedTask, expectedModel, expectedDim, false)
	if err != nil || len(updates) == 0 {
		return
	}

	if err := s.traceStore.ApplyTraceEmbeddingUpdates(updates); err != nil {
		logging.Get(logging.CategoryStore).Warn("Trace embedding update failed: %v", err)
		return
	}

	if err := s.syncTraceVectorIndex(updates, expectedDim); err != nil {
		logging.Get(logging.CategoryStore).Warn("Trace vector index update failed: %v", err)
	}

	logging.StoreDebug("Reflection trace embedder updated %d traces (embedded=%d)", len(updates), embedCount)
}

func (s *LocalStore) buildTraceEmbeddingUpdates(ctx context.Context, candidates []TraceEmbeddingCandidate, engine embedding.EmbeddingEngine, expectedTask, expectedModel string, expectedDim int, forceEmbed bool) ([]TraceEmbeddingUpdate, int, error) {
	taskAware, hasTaskAware := engine.(TaskTypeAwareEngine)

	type embedTarget struct {
		idx  int
		text string
	}

	updates := make([]TraceEmbeddingUpdate, 0, len(candidates))
	targets := make([]embedTarget, 0, len(candidates))

	for _, c := range candidates {
		desc := strings.TrimSpace(c.SummaryDescriptor)
		descVersion := c.DescriptorVersion
		descHash := strings.TrimSpace(c.DescriptorHash)
		descUpdated := false

		if desc == "" || descVersion != traceDescriptorVersion {
			desc = buildTraceDescriptor(c)
			descVersion = traceDescriptorVersion
			descUpdated = true
		}
		if desc != "" {
			computedHash := computeDescriptorHash(desc)
			if descHash == "" || computedHash != descHash {
				descHash = computedHash
				descUpdated = true
			}
		}

		update := TraceEmbeddingUpdate{
			ID:                c.ID,
			SummaryDescriptor: desc,
			DescriptorVersion: descVersion,
			DescriptorHash:    descHash,
			Embedding:         c.Embedding,
			EmbeddingModelID:  c.EmbeddingModelID,
			EmbeddingDim:      c.EmbeddingDim,
			EmbeddingTask:     c.EmbeddingTask,
		}

		needEmbed := forceEmbed || descUpdated || len(c.Embedding) == 0 || c.EmbeddingModelID != expectedModel || c.EmbeddingDim != expectedDim || c.EmbeddingTask != expectedTask
		if needEmbed && desc != "" {
			update.Embedding = nil
			update.EmbeddingModelID = ""
			update.EmbeddingDim = 0
			update.EmbeddingTask = ""
			targets = append(targets, embedTarget{idx: len(updates), text: desc})
		}

		updates = append(updates, update)
	}

	embedded := 0
	if len(targets) == 0 {
		return updates, embedded, nil
	}

	if hasTaskAware {
		for _, target := range targets {
			vec, err := taskAware.EmbedWithTask(ctx, target.text, expectedTask)
			if err != nil {
				continue
			}
			if len(vec) == 0 {
				continue
			}
			updates[target.idx].Embedding = encodeFloat32Slice(vec)
			updates[target.idx].EmbeddingModelID = expectedModel
			updates[target.idx].EmbeddingDim = len(vec)
			updates[target.idx].EmbeddingTask = expectedTask
			embedded++
		}
		return updates, embedded, nil
	}

	texts := make([]string, len(targets))
	for i, target := range targets {
		texts[i] = target.text
	}
	vecs, err := engine.EmbedBatch(ctx, texts)
	if err != nil {
		vecs = make([][]float32, len(targets))
		for i, target := range targets {
			vec, embedErr := engine.Embed(ctx, target.text)
			if embedErr != nil {
				continue
			}
			vecs[i] = vec
		}
	}
	for i, target := range targets {
		vec := vecs[i]
		if len(vec) == 0 {
			continue
		}
		updates[target.idx].Embedding = encodeFloat32Slice(vec)
		updates[target.idx].EmbeddingModelID = expectedModel
		updates[target.idx].EmbeddingDim = len(vec)
		updates[target.idx].EmbeddingTask = expectedTask
		embedded++
	}

	return updates, embedded, nil
}

func (s *LocalStore) syncTraceVectorIndex(updates []TraceEmbeddingUpdate, dim int) error {
	if s == nil || s.db == nil || !s.vectorExt {
		return nil
	}
	if dim <= 0 {
		return nil
	}
	if err := s.ensureTraceVecTable(dim); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	deleteStmt, err := tx.Prepare("DELETE FROM reasoning_traces_vec WHERE trace_id = ?")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	insertStmt, err := tx.Prepare("INSERT INTO reasoning_traces_vec (trace_id, embedding) VALUES (?, ?)")
	if err != nil {
		_ = deleteStmt.Close()
		_ = tx.Rollback()
		return err
	}

	for _, update := range updates {
		if _, err := deleteStmt.Exec(update.ID); err != nil {
			_ = tx.Rollback()
			_ = deleteStmt.Close()
			_ = insertStmt.Close()
			return err
		}
		if len(update.Embedding) == 0 {
			continue
		}
		if _, err := insertStmt.Exec(update.ID, update.Embedding); err != nil {
			_ = tx.Rollback()
			_ = deleteStmt.Close()
			_ = insertStmt.Close()
			return err
		}
	}

	_ = deleteStmt.Close()
	_ = insertStmt.Close()
	return tx.Commit()
}

func (s *LocalStore) ensureTraceVecTable(dim int) error {
	if s.db == nil {
		return fmt.Errorf("no database")
	}
	query := fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS reasoning_traces_vec USING vec0(embedding float[%d], trace_id TEXT)", dim)
	if _, err := s.db.Exec(query); err != nil {
		return err
	}
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_reasoning_traces_vec_trace_id ON reasoning_traces_vec(trace_id)"); err != nil {
		return err
	}
	return nil
}

// SetEmbeddingEngine configures the embedding engine for learning reflection.
func (ls *LearningStore) SetEmbeddingEngine(engine embedding.EmbeddingEngine) {
	if ls == nil {
		return
	}
	ls.mu.Lock()
	ls.embeddingEngine = engine
	ls.mu.Unlock()

	ls.startLearningReflectionWorker()
}

// SetReflectionConfig configures reflection embedding for the LearningStore.
func (ls *LearningStore) SetReflectionConfig(cfg config.ReflectionConfig) {
	if ls == nil {
		return
	}
	ls.mu.Lock()
	if !cfg.Enabled {
		ls.mu.Unlock()
		ls.stopLearningReflectionWorker()
		return
	}
	ls.mu.Unlock()
	ls.startLearningReflectionWorker()
}

func (ls *LearningStore) startLearningReflectionWorker() {
	if ls == nil {
		return
	}
	ls.mu.Lock()
	if ls.workerStop != nil {
		ls.mu.Unlock()
		return
	}
	if ls.embeddingEngine == nil {
		ls.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	ls.workerStop = stop
	ls.workerDone = done
	ls.mu.Unlock()

	go ls.runLearningReflectionWorker(stop, done)
}

func (ls *LearningStore) stopLearningReflectionWorker() {
	if ls == nil {
		return
	}
	ls.mu.Lock()
	stop := ls.workerStop
	done := ls.workerDone
	ls.workerStop = nil
	ls.workerDone = nil
	ls.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (ls *LearningStore) runLearningReflectionWorker(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(reflectionWorkerInterval)
	defer ticker.Stop()

	ls.processLearningReflectionCycle()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ls.processLearningReflectionCycle()
		}
	}
}

func (ls *LearningStore) processLearningReflectionCycle() {
	if ls == nil {
		return
	}

	ls.mu.RLock()
	engine := ls.embeddingEngine
	ls.mu.RUnlock()
	if engine == nil {
		return
	}

	expectedTask := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	expectedModel := engine.Name()
	expectedDim := engine.Dimensions()

	shardTypes := ls.listShardTypes()
	if len(shardTypes) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, shardType := range shardTypes {
		backlog := 0
		if count, err := ls.CountLearningEmbeddingBacklog(shardType, expectedModel, expectedDim, expectedTask); err == nil {
			backlog = count
		}
		batchSize := reflectionBatchSize
		if backlog > 0 && batchSize > 16 {
			batchSize = 16
		}

		candidates, err := ls.ListLearningEmbeddingCandidates(shardType, batchSize, expectedModel, expectedDim, expectedTask)
		if err != nil || len(candidates) == 0 {
			continue
		}

		updates, embedCount, err := ls.buildLearningEmbeddingUpdates(ctx, candidates, engine, expectedTask, expectedModel, expectedDim, false)
		if err != nil || len(updates) == 0 {
			continue
		}

		if err := ls.ApplyLearningEmbeddingUpdates(shardType, updates); err != nil {
			logging.Get(logging.CategoryStore).Warn("Learning embedding update failed for %s: %v", shardType, err)
			continue
		}

		db, err := ls.getDB(shardType)
		if err != nil {
			continue
		}
		if err := syncLearningVectorIndex(db, updates, expectedDim); err != nil {
			logging.Get(logging.CategoryStore).Warn("Learning vector index update failed for %s: %v", shardType, err)
		}

		logging.StoreDebug("Reflection learning embedder updated %d learnings for %s (embedded=%d)", len(updates), shardType, embedCount)
	}
}

func (ls *LearningStore) buildLearningEmbeddingUpdates(ctx context.Context, candidates []LearningEmbeddingCandidate, engine embedding.EmbeddingEngine, expectedTask, expectedModel string, expectedDim int, forceEmbed bool) ([]LearningEmbeddingUpdate, int, error) {
	taskAware, hasTaskAware := engine.(TaskTypeAwareEngine)

	type embedTarget struct {
		idx  int
		text string
	}

	updates := make([]LearningEmbeddingUpdate, 0, len(candidates))
	targets := make([]embedTarget, 0, len(candidates))

	for _, c := range candidates {
		handle := strings.TrimSpace(c.SemanticHandle)
		handleVersion := c.HandleVersion
		handleHash := strings.TrimSpace(c.HandleHash)
		handleUpdated := false

		if handle == "" || handleVersion != learningHandleVersion {
			handle = buildLearningHandle(c.ShardType, c.FactPredicate, c.FactArgs)
			handleVersion = learningHandleVersion
			handleUpdated = true
		}
		if handle != "" {
			computedHash := computeDescriptorHash(handle)
			if handleHash == "" || computedHash != handleHash {
				handleHash = computedHash
				handleUpdated = true
			}
		}

		update := LearningEmbeddingUpdate{
			ID:               c.ID,
			SemanticHandle:   handle,
			HandleVersion:    handleVersion,
			HandleHash:       handleHash,
			Embedding:        c.Embedding,
			EmbeddingModelID: c.EmbeddingModelID,
			EmbeddingDim:     c.EmbeddingDim,
			EmbeddingTask:    c.EmbeddingTask,
		}

		needEmbed := forceEmbed || handleUpdated || len(c.Embedding) == 0 || c.EmbeddingModelID != expectedModel || c.EmbeddingDim != expectedDim || c.EmbeddingTask != expectedTask
		if needEmbed && handle != "" {
			update.Embedding = nil
			update.EmbeddingModelID = ""
			update.EmbeddingDim = 0
			update.EmbeddingTask = ""
			targets = append(targets, embedTarget{idx: len(updates), text: handle})
		}

		updates = append(updates, update)
	}

	embedded := 0
	if len(targets) == 0 {
		return updates, embedded, nil
	}

	if hasTaskAware {
		for _, target := range targets {
			vec, err := taskAware.EmbedWithTask(ctx, target.text, expectedTask)
			if err != nil {
				continue
			}
			if len(vec) == 0 {
				continue
			}
			updates[target.idx].Embedding = encodeFloat32Slice(vec)
			updates[target.idx].EmbeddingModelID = expectedModel
			updates[target.idx].EmbeddingDim = len(vec)
			updates[target.idx].EmbeddingTask = expectedTask
			embedded++
		}
		return updates, embedded, nil
	}

	texts := make([]string, len(targets))
	for i, target := range targets {
		texts[i] = target.text
	}
	vecs, err := engine.EmbedBatch(ctx, texts)
	if err != nil {
		vecs = make([][]float32, len(targets))
		for i, target := range targets {
			vec, embedErr := engine.Embed(ctx, target.text)
			if embedErr != nil {
				continue
			}
			vecs[i] = vec
		}
	}
	for i, target := range targets {
		vec := vecs[i]
		if len(vec) == 0 {
			continue
		}
		updates[target.idx].Embedding = encodeFloat32Slice(vec)
		updates[target.idx].EmbeddingModelID = expectedModel
		updates[target.idx].EmbeddingDim = len(vec)
		updates[target.idx].EmbeddingTask = expectedTask
		embedded++
	}

	return updates, embedded, nil
}

func syncLearningVectorIndex(db *sql.DB, updates []LearningEmbeddingUpdate, dim int) error {
	if db == nil || dim <= 0 {
		return nil
	}
	if err := ensureLearningVecTable(db, dim); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	deleteStmt, err := tx.Prepare("DELETE FROM learnings_vec WHERE learning_id = ?")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	insertStmt, err := tx.Prepare("INSERT INTO learnings_vec (learning_id, embedding) VALUES (?, ?)")
	if err != nil {
		_ = deleteStmt.Close()
		_ = tx.Rollback()
		return err
	}

	for _, update := range updates {
		if _, err := deleteStmt.Exec(update.ID); err != nil {
			_ = tx.Rollback()
			_ = deleteStmt.Close()
			_ = insertStmt.Close()
			return err
		}
		if len(update.Embedding) == 0 {
			continue
		}
		if _, err := insertStmt.Exec(update.ID, update.Embedding); err != nil {
			_ = tx.Rollback()
			_ = deleteStmt.Close()
			_ = insertStmt.Close()
			return err
		}
	}

	_ = deleteStmt.Close()
	_ = insertStmt.Close()
	return tx.Commit()
}

func ensureLearningVecTable(db *sql.DB, dim int) error {
	if db == nil {
		return fmt.Errorf("no database")
	}
	query := fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS learnings_vec USING vec0(embedding float[%d], learning_id INTEGER)", dim)
	if _, err := db.Exec(query); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_learnings_vec_learning_id ON learnings_vec(learning_id)"); err != nil {
		return err
	}
	return nil
}

func applyRecencyWeight(score float64, createdAt time.Time, halfLifeDays int) float64 {
	if score <= 0 {
		return score
	}
	if createdAt.IsZero() || halfLifeDays <= 0 {
		return score
	}
	ageDays := time.Since(createdAt).Hours() / 24
	if ageDays <= 0 {
		return score
	}
	decay := math.Pow(0.5, ageDays/float64(halfLifeDays))
	return clampScore(score * decay)
}
