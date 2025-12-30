package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// MCPToolStore provides SQLite-backed storage for MCP servers and tools.
type MCPToolStore struct {
	mu sync.RWMutex

	db        *sql.DB
	embedder  embedding.EmbeddingEngine
	vectorExt bool   // True if sqlite-vec extension is available
	vecDims   int    // Vector dimensions
	dbPath    string // Path to database file
}

// NewMCPToolStore creates a new MCP tool store.
func NewMCPToolStore(dbPath string, embedder embedding.EmbeddingEngine) (*MCPToolStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &MCPToolStore{
		db:       db,
		embedder: embedder,
		dbPath:   dbPath,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the database schema.
func (s *MCPToolStore) initialize() error {
	// Create mcp_servers table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS mcp_servers (
			server_id TEXT PRIMARY KEY,
			endpoint TEXT NOT NULL,
			protocol TEXT NOT NULL,
			name TEXT,
			version TEXT,
			status TEXT DEFAULT 'unknown',
			capabilities TEXT,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_connected DATETIME,
			config TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create mcp_servers table: %w", err)
	}

	// Create mcp_tools table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS mcp_tools (
			tool_id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			input_schema TEXT,
			output_schema TEXT,

			categories TEXT,
			capabilities TEXT,
			domain TEXT,
			shard_affinities TEXT,
			use_cases TEXT,
			condensed TEXT,

			embedding BLOB,
			embedding_model TEXT,

			usage_count INTEGER DEFAULT 0,
			success_count INTEGER DEFAULT 0,
			avg_latency_ms INTEGER DEFAULT 0,
			last_used DATETIME,

			registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			analyzed_at DATETIME,

			UNIQUE(server_id, name)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create mcp_tools table: %w", err)
	}

	// Create indexes
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_tools_server ON mcp_tools(server_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_tools_category ON mcp_tools(categories)`)

	// Try to initialize vector extension
	s.initVectorExtension()

	return nil
}

// initVectorExtension attempts to initialize sqlite-vec for vector search.
func (s *MCPToolStore) initVectorExtension() {
	if s.embedder == nil {
		logging.Get(logging.CategoryTools).Debug("No embedder configured, vector search disabled")
		return
	}

	dims := s.embedder.Dimensions()
	s.vecDims = dims

	// Test if sqlite-vec is available
	testQuery := "CREATE VIRTUAL TABLE IF NOT EXISTS mcp_vec_probe USING vec0(embedding float[4])"
	if _, err := s.db.Exec(testQuery); err != nil {
		logging.Get(logging.CategoryTools).Debug("sqlite-vec not available: %v", err)
		s.vectorExt = false
		return
	}
	_, _ = s.db.Exec("DROP TABLE IF EXISTS mcp_vec_probe")

	// Create vector index table
	vecTable := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS mcp_tool_vec USING vec0(
			tool_id TEXT PRIMARY KEY,
			embedding float[%d]
		)
	`, dims)

	if _, err := s.db.Exec(vecTable); err != nil {
		logging.Get(logging.CategoryTools).Warn("Failed to create mcp_tool_vec: %v", err)
		s.vectorExt = false
		return
	}

	s.vectorExt = true
	logging.Get(logging.CategoryTools).Info("MCP tool vector index initialized (%d dimensions)", dims)
}

// Close closes the database connection.
func (s *MCPToolStore) Close() error {
	return s.db.Close()
}

// SaveServer persists an MCP server to the database.
func (s *MCPToolStore) SaveServer(ctx context.Context, server *MCPServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	capsJSON, _ := json.Marshal(server.Capabilities)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_servers (server_id, endpoint, protocol, name, version, status, capabilities, discovered_at, last_connected, config)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			endpoint = excluded.endpoint,
			protocol = excluded.protocol,
			name = excluded.name,
			version = excluded.version,
			status = excluded.status,
			capabilities = excluded.capabilities,
			last_connected = excluded.last_connected,
			config = excluded.config
	`,
		server.ID, server.Endpoint, server.Protocol, server.Name, server.Version,
		server.Status, string(capsJSON), server.DiscoveredAt, server.LastConnected, server.Config,
	)
	return err
}

// UpdateServerStatus updates the status of an MCP server.
func (s *MCPToolStore) UpdateServerStatus(ctx context.Context, serverID string, status ServerStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		UPDATE mcp_servers SET status = ?, last_connected = ? WHERE server_id = ?
	`, status, time.Now(), serverID)
	return err
}

// GetServer retrieves an MCP server by ID.
func (s *MCPToolStore) GetServer(ctx context.Context, serverID string) (*MCPServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var server MCPServer
	var capsJSON string
	var discoveredAt, lastConnected sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT server_id, endpoint, protocol, name, version, status, capabilities, discovered_at, last_connected, config
		FROM mcp_servers WHERE server_id = ?
	`, serverID).Scan(
		&server.ID, &server.Endpoint, &server.Protocol, &server.Name, &server.Version,
		&server.Status, &capsJSON, &discoveredAt, &lastConnected, &server.Config,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if capsJSON != "" {
		_ = json.Unmarshal([]byte(capsJSON), &server.Capabilities)
	}
	if discoveredAt.Valid {
		server.DiscoveredAt = discoveredAt.Time
	}
	if lastConnected.Valid {
		server.LastConnected = lastConnected.Time
	}

	return &server, nil
}

// GetAllServers retrieves all MCP servers.
func (s *MCPToolStore) GetAllServers(ctx context.Context) ([]*MCPServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, endpoint, protocol, name, version, status, capabilities, discovered_at, last_connected, config
		FROM mcp_servers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*MCPServer
	for rows.Next() {
		var server MCPServer
		var capsJSON string
		var discoveredAt, lastConnected sql.NullTime

		if err := rows.Scan(
			&server.ID, &server.Endpoint, &server.Protocol, &server.Name, &server.Version,
			&server.Status, &capsJSON, &discoveredAt, &lastConnected, &server.Config,
		); err != nil {
			return nil, err
		}

		if capsJSON != "" {
			_ = json.Unmarshal([]byte(capsJSON), &server.Capabilities)
		}
		if discoveredAt.Valid {
			server.DiscoveredAt = discoveredAt.Time
		}
		if lastConnected.Valid {
			server.LastConnected = lastConnected.Time
		}

		servers = append(servers, &server)
	}

	return servers, nil
}

// SaveTool persists an MCP tool to the database.
func (s *MCPToolStore) SaveTool(ctx context.Context, tool *MCPTool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	catsJSON, _ := json.Marshal(tool.Categories)
	capsJSON, _ := json.Marshal(tool.Capabilities)
	affinitiesJSON, _ := json.Marshal(tool.ShardAffinities)
	useCasesJSON, _ := json.Marshal(tool.UseCases)

	var embeddingBlob []byte
	if len(tool.Embedding) > 0 {
		embeddingBlob = float32SliceToBytes(tool.Embedding)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_tools (
			tool_id, server_id, name, description, input_schema, output_schema,
			categories, capabilities, domain, shard_affinities, use_cases, condensed,
			embedding, embedding_model, registered_at, analyzed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_id) DO UPDATE SET
			description = excluded.description,
			input_schema = excluded.input_schema,
			output_schema = excluded.output_schema,
			categories = excluded.categories,
			capabilities = excluded.capabilities,
			domain = excluded.domain,
			shard_affinities = excluded.shard_affinities,
			use_cases = excluded.use_cases,
			condensed = excluded.condensed,
			embedding = excluded.embedding,
			embedding_model = excluded.embedding_model,
			analyzed_at = excluded.analyzed_at
	`,
		tool.ToolID, tool.ServerID, tool.Name, tool.Description,
		string(tool.InputSchema), string(tool.OutputSchema),
		string(catsJSON), string(capsJSON), tool.Domain, string(affinitiesJSON),
		string(useCasesJSON), tool.Condensed,
		embeddingBlob, tool.EmbeddingModel, tool.RegisteredAt, tool.AnalyzedAt,
	)
	if err != nil {
		return err
	}

	// Update vector index if available
	if s.vectorExt && len(tool.Embedding) > 0 {
		s.updateVectorIndex(ctx, tool.ToolID, tool.Embedding)
	}

	return nil
}

// updateVectorIndex updates the vector index for a tool.
func (s *MCPToolStore) updateVectorIndex(ctx context.Context, toolID string, embedding []float32) {
	// Delete existing entry
	_, _ = s.db.ExecContext(ctx, "DELETE FROM mcp_tool_vec WHERE tool_id = ?", toolID)

	// Insert new entry
	embeddingBlob := float32SliceToBytes(embedding)
	_, err := s.db.ExecContext(ctx, "INSERT INTO mcp_tool_vec (tool_id, embedding) VALUES (?, ?)", toolID, embeddingBlob)
	if err != nil {
		logging.Get(logging.CategoryTools).Debug("Failed to update vector index for %s: %v", toolID, err)
	}
}

// GetTool retrieves an MCP tool by ID.
func (s *MCPToolStore) GetTool(ctx context.Context, toolID string) (*MCPTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getToolLocked(ctx, toolID)
}

// getToolLocked retrieves a tool (caller must hold lock).
func (s *MCPToolStore) getToolLocked(ctx context.Context, toolID string) (*MCPTool, error) {
	var tool MCPTool
	var inputSchema, outputSchema, catsJSON, capsJSON, affinitiesJSON, useCasesJSON sql.NullString
	var embeddingBlob []byte
	var registeredAt, analyzedAt, lastUsed sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT tool_id, server_id, name, description, input_schema, output_schema,
			categories, capabilities, domain, shard_affinities, use_cases, condensed,
			embedding, embedding_model, usage_count, success_count, avg_latency_ms, last_used,
			registered_at, analyzed_at
		FROM mcp_tools WHERE tool_id = ?
	`, toolID).Scan(
		&tool.ToolID, &tool.ServerID, &tool.Name, &tool.Description,
		&inputSchema, &outputSchema, &catsJSON, &capsJSON, &tool.Domain,
		&affinitiesJSON, &useCasesJSON, &tool.Condensed,
		&embeddingBlob, &tool.EmbeddingModel, &tool.UsageCount, &tool.SuccessCount,
		&tool.AvgLatencyMs, &lastUsed, &registeredAt, &analyzedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if inputSchema.Valid {
		tool.InputSchema = json.RawMessage(inputSchema.String)
	}
	if outputSchema.Valid {
		tool.OutputSchema = json.RawMessage(outputSchema.String)
	}
	if catsJSON.Valid {
		_ = json.Unmarshal([]byte(catsJSON.String), &tool.Categories)
	}
	if capsJSON.Valid {
		_ = json.Unmarshal([]byte(capsJSON.String), &tool.Capabilities)
	}
	if affinitiesJSON.Valid {
		_ = json.Unmarshal([]byte(affinitiesJSON.String), &tool.ShardAffinities)
	}
	if useCasesJSON.Valid {
		_ = json.Unmarshal([]byte(useCasesJSON.String), &tool.UseCases)
	}
	if len(embeddingBlob) > 0 {
		tool.Embedding = bytesToFloat32Slice(embeddingBlob)
	}
	if registeredAt.Valid {
		tool.RegisteredAt = registeredAt.Time
	}
	if analyzedAt.Valid {
		tool.AnalyzedAt = analyzedAt.Time
	}
	if lastUsed.Valid {
		tool.LastUsed = lastUsed.Time
	}

	return &tool, nil
}

// GetAllTools retrieves all MCP tools.
func (s *MCPToolStore) GetAllTools(ctx context.Context) ([]*MCPTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_id FROM mcp_tools
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*MCPTool
	for rows.Next() {
		var toolID string
		if err := rows.Scan(&toolID); err != nil {
			return nil, err
		}
		tool, err := s.getToolLocked(ctx, toolID)
		if err != nil {
			return nil, err
		}
		if tool != nil {
			tools = append(tools, tool)
		}
	}

	return tools, nil
}

// GetToolsByServer retrieves all tools for a specific server.
func (s *MCPToolStore) GetToolsByServer(ctx context.Context, serverID string) ([]*MCPTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_id FROM mcp_tools WHERE server_id = ?
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*MCPTool
	for rows.Next() {
		var toolID string
		if err := rows.Scan(&toolID); err != nil {
			return nil, err
		}
		tool, err := s.getToolLocked(ctx, toolID)
		if err != nil {
			return nil, err
		}
		if tool != nil {
			tools = append(tools, tool)
		}
	}

	return tools, nil
}

// RecordToolUsage records a tool usage event.
func (s *MCPToolStore) RecordToolUsage(ctx context.Context, toolID string, success bool, latencyMs int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	successInc := 0
	if success {
		successInc = 1
	}

	// Update counts and running average latency
	_, err := s.db.ExecContext(ctx, `
		UPDATE mcp_tools SET
			usage_count = usage_count + 1,
			success_count = success_count + ?,
			avg_latency_ms = ((avg_latency_ms * usage_count) + ?) / (usage_count + 1),
			last_used = ?
		WHERE tool_id = ?
	`, successInc, latencyMs, time.Now(), toolID)
	return err
}

// SemanticSearch finds tools semantically similar to the query.
func (s *MCPToolStore) SemanticSearch(ctx context.Context, queryEmbedding []float32, topK int) ([]ToolSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.vectorExt {
		return s.semanticSearchVec(ctx, queryEmbedding, topK)
	}
	return s.semanticSearchBruteForce(ctx, queryEmbedding, topK)
}

// ToolSearchResult represents a semantic search result.
type ToolSearchResult struct {
	ToolID string
	Score  float64 // 0.0 to 1.0 similarity
}

// semanticSearchVec uses sqlite-vec for fast ANN search.
func (s *MCPToolStore) semanticSearchVec(ctx context.Context, queryEmbedding []float32, topK int) ([]ToolSearchResult, error) {
	embeddingBlob := float32SliceToBytes(queryEmbedding)

	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_id, vec_distance_cosine(embedding, ?) as distance
		FROM mcp_tool_vec
		ORDER BY distance
		LIMIT ?
	`, embeddingBlob, topK)
	if err != nil {
		logging.Get(logging.CategoryTools).Debug("vec search failed, falling back: %v", err)
		return s.semanticSearchBruteForce(ctx, queryEmbedding, topK)
	}
	defer rows.Close()

	var results []ToolSearchResult
	for rows.Next() {
		var toolID string
		var distance float64
		if err := rows.Scan(&toolID, &distance); err != nil {
			continue
		}
		// Convert distance to similarity (1 - distance for cosine)
		score := 1.0 - distance
		if score < 0 {
			score = 0
		}
		results = append(results, ToolSearchResult{
			ToolID: toolID,
			Score:  score,
		})
	}

	return results, nil
}

// semanticSearchBruteForce uses brute-force cosine similarity.
func (s *MCPToolStore) semanticSearchBruteForce(ctx context.Context, queryEmbedding []float32, topK int) ([]ToolSearchResult, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tool_id, embedding FROM mcp_tools WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ToolSearchResult
	for rows.Next() {
		var toolID string
		var embeddingBlob []byte
		if err := rows.Scan(&toolID, &embeddingBlob); err != nil {
			continue
		}
		if len(embeddingBlob) == 0 {
			continue
		}

		embedding := bytesToFloat32Slice(embeddingBlob)
		score := cosineSimilarity(queryEmbedding, embedding)
		results = append(results, ToolSearchResult{
			ToolID: toolID,
			Score:  score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// Helper functions for embedding serialization

func float32SliceToBytes(floats []float32) []byte {
	bytes := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		bytes[i*4] = byte(bits)
		bytes[i*4+1] = byte(bits >> 8)
		bytes[i*4+2] = byte(bits >> 16)
		bytes[i*4+3] = byte(bits >> 24)
	}
	return bytes
}

func bytesToFloat32Slice(bytes []byte) []float32 {
	floats := make([]float32, len(bytes)/4)
	for i := range floats {
		bits := uint32(bytes[i*4]) | uint32(bytes[i*4+1])<<8 | uint32(bytes[i*4+2])<<16 | uint32(bytes[i*4+3])<<24
		floats[i] = math.Float32frombits(bits)
	}
	return floats
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
