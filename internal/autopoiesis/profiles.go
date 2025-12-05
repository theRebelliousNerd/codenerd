package autopoiesis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// TOOL QUALITY PROFILE - LLM-DEFINED EXPECTATIONS PER TOOL
// =============================================================================
// Each tool has different expectations. A background indexer that takes
// 5 minutes is fine; a calculator that takes 5 minutes is broken.
// The LLM defines these expectations during tool generation.

// ToolQualityProfile defines expected quality dimensions for a specific tool
type ToolQualityProfile struct {
	// Identity
	ToolName    string    `json:"tool_name"`
	ToolType    ToolType  `json:"tool_type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`

	// Performance Expectations
	Performance PerformanceExpectations `json:"performance"`

	// Output Expectations
	Output OutputExpectations `json:"output"`

	// Usage Pattern
	UsagePattern UsagePattern `json:"usage_pattern"`

	// Caching Configuration
	Caching CachingConfig `json:"caching"`

	// Custom Dimensions (tool-specific metrics)
	CustomDimensions []CustomDimension `json:"custom_dimensions,omitempty"`
}

// ToolType classifies the tool for appropriate evaluation
type ToolType string

const (
	ToolTypeQuickCalculation  ToolType = "quick_calculation"  // < 1s, simple computation
	ToolTypeDataFetch         ToolType = "data_fetch"         // API call, may paginate
	ToolTypeBackgroundTask    ToolType = "background_task"    // Long-running, minutes OK
	ToolTypeRecursiveAnalysis ToolType = "recursive_analysis" // Codebase traversal, slow OK
	ToolTypeRealTimeQuery     ToolType = "realtime_query"     // Must be fast, frequent
	ToolTypeOneTimeSetup      ToolType = "one_time_setup"     // Run once, can be slow
	ToolTypeBatchProcessor    ToolType = "batch_processor"    // Processes many items
	ToolTypeMonitor           ToolType = "monitor"            // Called repeatedly for status
)

// PerformanceExpectations defines expected timing for this tool
type PerformanceExpectations struct {
	// Duration expectations
	ExpectedDurationMin time.Duration `json:"expected_duration_min"` // Faster than this is suspicious
	ExpectedDurationMax time.Duration `json:"expected_duration_max"` // Slower than this is a problem
	AcceptableDuration  time.Duration `json:"acceptable_duration"`   // Target duration
	TimeoutDuration     time.Duration `json:"timeout_duration"`      // When to give up

	// Resource expectations
	MaxMemoryMB      int64   `json:"max_memory_mb,omitempty"`
	ExpectedAPIcalls int     `json:"expected_api_calls,omitempty"` // Expected number of external calls
	MaxRetries       int     `json:"max_retries"`                  // How many retries are acceptable

	// Scaling behavior
	ScalesWithInputSize bool    `json:"scales_with_input_size"` // Duration scales with input?
	ScalingFactor       float64 `json:"scaling_factor,omitempty"` // ms per unit of input size
}

// OutputExpectations defines what output should look like
type OutputExpectations struct {
	// Size expectations
	ExpectedMinSize     int `json:"expected_min_size"`     // Smaller is suspicious
	ExpectedMaxSize     int `json:"expected_max_size"`     // Larger might indicate issue
	ExpectedTypicalSize int `json:"expected_typical_size"` // Normal output size

	// Content expectations
	ExpectedFormat string   `json:"expected_format"`               // json, text, csv, etc.
	RequiredFields []string `json:"required_fields,omitempty"`     // Fields that must be present
	MustContain    []string `json:"must_contain,omitempty"`        // Strings that must appear
	MustNotContain []string `json:"must_not_contain,omitempty"`    // Strings that indicate failure

	// Pagination expectations
	ExpectsPagination bool `json:"expects_pagination"`            // Should we paginate?
	ExpectedPages     int  `json:"expected_pages,omitempty"`      // How many pages expected

	// Completeness criteria
	CompletenessCheck string `json:"completeness_check,omitempty"` // How to verify completeness
}

// UsagePattern describes how this tool is typically used
type UsagePattern struct {
	Frequency         UsageFrequency `json:"frequency"`           // How often called
	CallsPerSession   int            `json:"calls_per_session"`   // Expected calls per session
	IsIdempotent      bool           `json:"is_idempotent"`       // Same input = same output?
	HasSideEffects    bool           `json:"has_side_effects"`    // Modifies external state?
	DependsOnExternal bool           `json:"depends_on_external"` // Needs external service?
}

// UsageFrequency describes how often a tool is called
type UsageFrequency string

const (
	FrequencyOnce       UsageFrequency = "once"       // Run once per task
	FrequencyOccasional UsageFrequency = "occasional" // Few times per session
	FrequencyFrequent   UsageFrequency = "frequent"   // Many times per session
	FrequencyConstant   UsageFrequency = "constant"   // Called continuously
)

// CachingConfig defines caching behavior
type CachingConfig struct {
	Cacheable     bool          `json:"cacheable"`                   // Can results be cached?
	CacheDuration time.Duration `json:"cache_duration"`              // How long to cache
	CacheKey      string        `json:"cache_key"`                   // What makes cache key unique
	InvalidateOn  []string      `json:"invalidate_on,omitempty"`     // Events that invalidate cache
}

// CustomDimension allows tool-specific quality metrics
type CustomDimension struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ExpectedValue  float64 `json:"expected_value"`
	Tolerance      float64 `json:"tolerance"`       // +/- acceptable range
	Weight         float64 `json:"weight"`          // How much this affects overall score
	ExtractPattern string  `json:"extract_pattern"` // Regex to extract value from output
}

// ProfileStore manages tool quality profiles
type ProfileStore struct {
	mu        sync.RWMutex
	profiles  map[string]*ToolQualityProfile // ToolName -> Profile
	storePath string
}

// NewProfileStore creates a new profile store
func NewProfileStore(storePath string) *ProfileStore {
	store := &ProfileStore{
		profiles:  make(map[string]*ToolQualityProfile),
		storePath: storePath,
	}
	store.load()
	return store
}

// GetProfile retrieves a tool's quality profile
func (ps *ProfileStore) GetProfile(toolName string) *ToolQualityProfile {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.profiles[toolName]
}

// SetProfile stores a tool's quality profile
func (ps *ProfileStore) SetProfile(profile *ToolQualityProfile) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.profiles[profile.ToolName] = profile
	ps.save()
}

// GetDefaultProfile returns a default profile based on tool type
func GetDefaultProfile(toolName string, toolType ToolType) *ToolQualityProfile {
	profile := &ToolQualityProfile{
		ToolName:  toolName,
		ToolType:  toolType,
		CreatedAt: time.Now(),
	}

	// Set defaults based on tool type
	switch toolType {
	case ToolTypeQuickCalculation:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Millisecond,
			ExpectedDurationMax: 1 * time.Second,
			AcceptableDuration:  100 * time.Millisecond,
			TimeoutDuration:     5 * time.Second,
			MaxRetries:          0,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     1,
			ExpectedMaxSize:     1024,
			ExpectedTypicalSize: 100,
			ExpectsPagination:   false,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyFrequent,
			CallsPerSession: 50,
			IsIdempotent:    true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 5 * time.Minute,
		}

	case ToolTypeDataFetch:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 100 * time.Millisecond,
			ExpectedDurationMax: 30 * time.Second,
			AcceptableDuration:  5 * time.Second,
			TimeoutDuration:     60 * time.Second,
			ExpectedAPIcalls:    1,
			MaxRetries:          3,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     100,
			ExpectedMaxSize:     1024 * 1024, // 1MB
			ExpectedTypicalSize: 10 * 1024,   // 10KB
			ExpectsPagination:   true,
			ExpectedPages:       5,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyOccasional,
			CallsPerSession:   5,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 15 * time.Minute,
		}

	case ToolTypeBackgroundTask:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Second,
			ExpectedDurationMax: 10 * time.Minute,
			AcceptableDuration:  2 * time.Minute,
			TimeoutDuration:     30 * time.Minute,
			MaxRetries:          2,
			ScalesWithInputSize: true,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     10 * 1024 * 1024, // 10MB
			ExpectedTypicalSize: 100 * 1024,       // 100KB
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOnce,
			CallsPerSession: 1,
			HasSideEffects:  true,
		}
		profile.Caching = CachingConfig{
			Cacheable: false,
		}

	case ToolTypeRecursiveAnalysis:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 5 * time.Second,
			ExpectedDurationMax: 15 * time.Minute,
			AcceptableDuration:  3 * time.Minute,
			TimeoutDuration:     30 * time.Minute,
			MaxRetries:          1,
			ScalesWithInputSize: true,
			ScalingFactor:       10, // 10ms per file
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     1024,
			ExpectedMaxSize:     50 * 1024 * 1024, // 50MB
			ExpectedTypicalSize: 500 * 1024,       // 500KB
			ExpectsPagination:   false,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOnce,
			CallsPerSession: 1,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 1 * time.Hour,
		}

	case ToolTypeRealTimeQuery:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Millisecond,
			ExpectedDurationMax: 500 * time.Millisecond,
			AcceptableDuration:  100 * time.Millisecond,
			TimeoutDuration:     2 * time.Second,
			MaxRetries:          1,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     10 * 1024, // 10KB
			ExpectedTypicalSize: 1024,      // 1KB
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyConstant,
			CallsPerSession:   100,
			IsIdempotent:      true,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 1 * time.Minute,
		}

	case ToolTypeMonitor:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 10 * time.Millisecond,
			ExpectedDurationMax: 2 * time.Second,
			AcceptableDuration:  500 * time.Millisecond,
			TimeoutDuration:     5 * time.Second,
			MaxRetries:          2,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     50,
			ExpectedMaxSize:     5 * 1024, // 5KB
			ExpectedTypicalSize: 500,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyConstant,
			CallsPerSession:   200,
			IsIdempotent:      true,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 30 * time.Second,
		}

	default:
		// Generic defaults
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 100 * time.Millisecond,
			ExpectedDurationMax: 30 * time.Second,
			AcceptableDuration:  5 * time.Second,
			TimeoutDuration:     60 * time.Second,
			MaxRetries:          2,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     1024 * 1024,
			ExpectedTypicalSize: 10 * 1024,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOccasional,
			CallsPerSession: 10,
		}
	}

	return profile
}

// persistence
func (ps *ProfileStore) load() {
	path := filepath.Join(ps.storePath, "quality_profiles.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &ps.profiles)
}

func (ps *ProfileStore) save() {
	if err := os.MkdirAll(ps.storePath, 0755); err != nil {
		return
	}
	path := filepath.Join(ps.storePath, "quality_profiles.json")
	data, _ := json.MarshalIndent(ps.profiles, "", "  ")
	os.WriteFile(path, data, 0644)
}
