package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file groups the document ingestion and classification stage of the
// decomposer: reading source documents, classifying them, seeding facts, and
// persisting them into the per-campaign knowledge store.

// ingestSourceDocuments reads and parses source documents (metadata only).
func (d *Decomposer) ingestSourceDocuments(ctx context.Context, campaignID string, paths []string) ([]SourceDocument, []FileMetadata, error) {
	logging.CampaignDebug("Ingesting source documents from %d paths", len(paths))

	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	for _, path := range paths {
		// Check for cancellation between file reads
		select {
		case <-ctx.Done():
			logging.CampaignDebug("Source ingestion cancelled")
			return docs, meta, ctx.Err()
		default:
		}
		// Resolve path
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(d.workspace, path)
		}

		logging.CampaignDebug("Processing path: %s", fullPath)

		stat, err := os.Stat(fullPath)
		if err != nil {
			// Try glob pattern
			matches, _ := filepath.Glob(fullPath)
			if len(matches) == 0 {
				logging.CampaignDebug("Skipping missing path: %s", fullPath)
				continue // Skip missing files
			}
			logging.CampaignDebug("Glob matched %d files", len(matches))
			for _, match := range matches {
				mds, mmeta := d.readDocumentsFromPath(match, campaignID)
				docs = append(docs, mds...)
				meta = append(meta, mmeta...)
			}
			continue
		}

		if stat.IsDir() {
			logging.CampaignDebug("Reading directory: %s", fullPath)
			mds, mmeta := d.readDocumentsFromDir(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		} else {
			mds, mmeta := d.readDocumentsFromPath(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		}
	}

	logging.CampaignDebug("Classifying %d documents by architectural layer", len(meta))
	meta = d.classifyDocuments(ctx, meta)

	logging.CampaignDebug("Ingestion complete: docs=%d, meta=%d", len(docs), len(meta))
	return docs, meta, nil
}

// classifyDocuments routes files through the Librarian to assign architectural layers.
func (d *Decomposer) classifyDocuments(ctx context.Context, files []FileMetadata) []FileMetadata {
	if len(files) == 0 {
		return files
	}

	if d.llmClient == nil {
		logging.CampaignDebug("No LLM client, using default layer classification")
		for i := range files {
			if files[i].Layer == "" {
				files[i].Layer = "/scaffold"
			}
			if files[i].LayerConfidence == 0 {
				files[i].LayerConfidence = 0.1
			}
		}
		return files
	}

	classifiedCount := 0
	for i := range files {
		select {
		case <-ctx.Done():
			logging.CampaignDebug("Document classification cancelled after %d files", classifiedCount)
			return files
		default:
		}

		// Sensible defaults if classification is unavailable
		files[i].Layer = "/scaffold"
		files[i].LayerConfidence = 0.1

		if files[i].SizeBytes > maxCampaignClassificationBytes {
			logging.CampaignDebug("Skipping classification for oversized file: %s (%d bytes)", files[i].Path, files[i].SizeBytes)
			continue
		}

		data, err := os.ReadFile(files[i].Path)
		if err != nil {
			logging.CampaignDebug("Cannot read file for classification: %s", files[i].Path)
			continue
		}

		class, err := d.classifyDocument(ctx, files[i].Path, string(data))
		if err != nil {
			logging.CampaignDebug("Classification failed for %s: %v", files[i].Path, err)
			continue
		}

		if class.Layer != "" {
			files[i].Layer = class.Layer
		}
		if class.Confidence > 0 {
			files[i].LayerConfidence = class.Confidence
		}
		if class.Reasoning != "" {
			files[i].LayerReason = class.Reasoning
		}
		classifiedCount++
		logging.CampaignDebug("Classified %s -> %s (confidence=%.2f)",
			filepath.Base(files[i].Path), files[i].Layer, files[i].LayerConfidence)
	}

	logging.CampaignDebug("Classified %d/%d documents", classifiedCount, len(files))
	return files
}

func (d *Decomposer) readDocumentsFromDir(dir string, campaignID string) ([]SourceDocument, []FileMetadata) {
	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !isSupportedDocExt(path) {
			return nil
		}
		mds, mmeta := d.readDocumentsFromPath(path, campaignID)
		docs = append(docs, mds...)
		meta = append(meta, mmeta...)
		return nil
	})

	return docs, meta
}

func (d *Decomposer) readDocumentsFromPath(path string, campaignID string) ([]SourceDocument, []FileMetadata) {
	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	docType := d.inferDocType(path)
	stat, err := os.Stat(path)
	if err != nil {
		return docs, meta
	}

	docs = append(docs, SourceDocument{
		CampaignID: campaignID,
		Path:       path,
		Type:       docType,
		ParsedAt:   time.Now(),
	})
	tags := deriveTagsFromPath(path)
	meta = append(meta, FileMetadata{
		Path:       path,
		Type:       docType,
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime(),
		Tags:       tags,
	})
	return docs, meta
}

// classifyDocument asks the LLM to bucket the document into an architectural layer.
func (d *Decomposer) classifyDocument(ctx context.Context, filename, content string) (DocClassification, error) {
	defaultClass := DocClassification{Layer: "/scaffold", Confidence: 0.1}

	if d.llmClient == nil {
		return defaultClass, nil
	}

	trimmed := strings.TrimSpace(content)
	lowerName := strings.ToLower(filename)

	// Optimization: Don't classify trivial files
	if len(trimmed) < 50 || strings.HasSuffix(lowerName, ".txt") {
		return DocClassification{Layer: "/scaffold", Confidence: 0.5, Reasoning: "defaulted (trivial content)"}, nil
	}

	// Get prompt (JIT or static)
	basePrompt, err := d.promptProvider.GetPrompt(ctx, RoleLibrarian, "")
	if err != nil {
		logging.CampaignDebug("Failed to get Librarian prompt, using fallback: %v", err)
		basePrompt = LibrarianLogic
	}

	prompt := fmt.Sprintf(`%s

FILE: %s
CONTENT START:
%s
CONTENT END

Return JSON only: {"layer": "/string", "confidence": 0.0-1.0, "reasoning": "brief"}`,
		basePrompt, filename, limitString(trimmed, 2000))

	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		return defaultClass, err
	}

	var result DocClassification
	if err := json.Unmarshal([]byte(cleanJSONResponse(resp)), &result); err != nil {
		return defaultClass, err
	}

	if result.Layer == "" {
		result.Layer = "/scaffold"
	}
	if result.Confidence == 0 {
		result.Confidence = defaultClass.Confidence
	}

	return result, nil
}

// ingestIntoKnowledgeStore persists all document chunks into the campaign knowledge DB (vectors + KG).
func (d *Decomposer) ingestIntoKnowledgeStore(ctx context.Context, campaignID, dbPath string, files []FileMetadata) error {
	if len(files) == 0 {
		logging.CampaignDebug("No files to ingest into knowledge store")
		return nil
	}

	logging.CampaignDebug("Initializing document ingestor: dbPath=%s", dbPath)
	ingestor, err := NewDocumentIngestor(dbPath, embedding.DefaultConfig())
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create document ingestor: %v", err)
		return err
	}
	defer ingestor.Close()

	ingestedCount := 0
	totalBytes := int64(0)
	for _, fm := range files {
		if fm.SizeBytes > maxCampaignKnowledgeIngestBytes {
			logging.CampaignDebug("Skipping knowledge ingestion for oversized file: %s (%d bytes)", fm.Path, fm.SizeBytes)
			continue
		}

		data, err := os.ReadFile(fm.Path)
		if err != nil {
			logging.CampaignDebug("Failed to read file for ingestion: %s - %v", fm.Path, err)
			continue
		}
		payload := map[string]string{fm.Path: string(data)}
		_, _ = ingestor.Ingest(ctx, campaignID, payload)
		ingestedCount++
		totalBytes += int64(len(data))
	}

	logging.Campaign("Knowledge store ingestion complete: files=%d, bytes=%d", ingestedCount, totalBytes)
	return nil
}

// seedDocFacts pushes lightweight document metadata into the kernel for logic-based selection.
func (d *Decomposer) seedDocFacts(campaignID, goal string, files []FileMetadata) {
	if d.kernel == nil {
		logging.CampaignDebug("No kernel available for seeding doc facts")
		return
	}

	logging.CampaignDebug("Seeding %d document facts for campaign %s", len(files), campaignID)
	facts := make([]core.Fact, 0, len(files)+1)
	// Campaign goal fact already loaded later; still record a preliminary goal signal for selection rules.
	facts = append(facts, core.Fact{
		Predicate: "campaign_goal",
		Args:      []any{campaignID, goal},
	})

	topics := extractTopicsFromGoal(goal)
	for _, topic := range topics {
		facts = append(facts, core.Fact{
			Predicate: "goal_topic",
			Args:      []any{campaignID, fmt.Sprintf("/%s", topic)},
		})
	}

	for _, fm := range files {
		facts = append(facts, core.Fact{
			Predicate: "doc_metadata",
			Args:      []any{campaignID, fm.Path, fm.Type, fm.SizeBytes, fm.ModifiedAt.Unix()},
		})
		layer := fm.Layer
		if layer == "" {
			layer = "/scaffold"
		}
		confidence := fm.LayerConfidence
		if confidence == 0 {
			confidence = 0.1
		}
		facts = append(facts, core.Fact{
			Predicate: "doc_layer",
			Args:      []any{fm.Path, layer, confidence},
		})
		for _, tag := range fm.Tags {
			facts = append(facts, core.Fact{
				Predicate: "doc_tag",
				Args:      []any{fm.Path, fmt.Sprintf("/%s", tag)},
			})
		}
	}

	if err := d.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"Failed to assert doc facts batch campaign=%s fact_count=%d: %v",
			campaignID, len(facts), err)
	} else {
		logging.CampaignDebug("Seeded %d facts into kernel", len(facts))
	}
}

// inferDocType infers the document type from filename.
func (d *Decomposer) inferDocType(path string) string {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(lower, "spec"):
		return "/spec"
	case strings.Contains(lower, "requirement"):
		return "/requirements"
	case strings.Contains(lower, "design"):
		return "/design"
	case strings.Contains(lower, "readme"):
		return "/readme"
	case strings.Contains(lower, "api"):
		return "/api_doc"
	case strings.Contains(lower, "tutorial"):
		return "/tutorial"
	default:
		return "/spec"
	}
}
