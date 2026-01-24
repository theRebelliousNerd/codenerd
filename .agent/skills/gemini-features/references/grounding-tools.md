# Grounding Tools Reference

## Overview

Gemini provides two built-in tools for grounding responses with external information:

1. **Google Search** - Real-time web search
2. **URL Context** - Ground responses with specific URLs

These tools CANNOT be combined with custom function declarations in the same request (Gemini API limitation). bit can be combined with structured output.

## Google Search Grounding

### Enabling Google Search

```go
type GeminiGoogleSearch struct {} // Empty struct - presence enables it

// Add to tools array
tools = append(tools, GeminiTool{
    GoogleSearch: &GeminiGoogleSearch{},
})
```

### Request Format

```json
{
  "contents": [{"parts": [{"text": "What are the latest Go 1.22 features?"}]}],
  "tools": [{"google_search": {}}],
  "generationConfig": {
    "thinkingConfig": {"includeThoughts": true, "thinkingLevel": "high"}
  }
}
```

### Response with Grounding Metadata

```go
type GeminiGroundingMetadata struct {
    GroundingChunks   []GeminiGroundingChunk   `json:"groundingChunks,omitempty"`
    GroundingSupports []GeminiGroundingSupport `json:"groundingSupports,omitempty"`
    WebSearchQueries  []string                 `json:"webSearchQueries,omitempty"`
    SearchEntryPoint  *GeminiSearchEntryPoint  `json:"searchEntryPoint,omitempty"`
}

type GeminiGroundingChunk struct {
    Web *struct {
        URI   string `json:"uri"`
        Title string `json:"title"`
    } `json:"web,omitempty"`
}

type GeminiGroundingSupport struct {
    Segment struct {
        StartIndex int    `json:"startIndex"`
        EndIndex   int    `json:"endIndex"`
        Text       string `json:"text"`
    } `json:"segment"`
    GroundingChunkIndices []int     `json:"groundingChunkIndices"`
    ConfidenceScores      []float64 `json:"confidenceScores"`
}
```

### Extracting Grounding Sources

```go
func extractGroundingSources(gm *GeminiGroundingMetadata) []string {
    if gm == nil {
        return nil
    }

    var sources []string
    for _, chunk := range gm.GroundingChunks {
        if chunk.Web != nil && chunk.Web.URI != "" {
            sources = append(sources, chunk.Web.URI)
        }
    }
    return sources
}
```

## URL Context Tool

Ground responses with specific documentation URLs. Ideal for:

- API documentation
- GitHub repositories
- Technical specifications
- Company wikis

### Configuration

```go
type GeminiURLContext struct {
    URLs []string `json:"urls,omitempty"` // Max 20 URLs, 34MB each
}

// Client configuration
func (c *GeminiClient) SetURLContextURLs(urls []string) {
    c.urlContextURLs = urls
}

func (c *GeminiClient) SetEnableURLContext(enable bool) {
    c.enableURLContext = enable
}
```

### Request Format

```json
{
  "contents": [{"parts": [{"text": "How do I use the Context7 API?"}]}],
  "tools": [{
    "url_context": {
      "urls": [
        "https://context7.dev/docs/api",
        "https://github.com/context7/sdk-go/blob/main/README.md"
      ]
    }
  }]
}
```

### Implementation

```go
func (c *GeminiClient) buildBuiltInTools() []GeminiTool {
    var tools []GeminiTool

    // Google Search
    if c.enableGoogleSearch {
        tools = append(tools, GeminiTool{
            GoogleSearch: &GeminiGoogleSearch{},
        })
    }

    // URL Context
    if c.enableURLContext && len(c.urlContextURLs) > 0 {
        urls := c.urlContextURLs
        if len(urls) > 20 {
            urls = urls[:20] // API limit
        }
        tools = append(tools, GeminiTool{
            URLContext: &GeminiURLContext{URLs: urls},
        })
    }

    return tools
}
```

### Limitations

| Limitation | Value |
|------------|-------|
| Max URLs | 20 |
| Max file size | 34 MB per URL |
| Paywalled content | Not accessible |
| YouTube videos | Not supported |
| Google Workspace | Not supported |

**Note:** URL content counts toward input tokens.

### Two-Step Retrieval

URL Context uses a two-step process:

1. **Cache check** - Looks for cached version of URL
2. **Live fetch** - Fetches fresh content if not cached

## Best Practices

### When to Use Google Search

- Current events and news
- Latest version information
- Real-time data (weather, stocks, etc.)
- General knowledge queries

### When to Use URL Context

- Specific documentation
- Private/internal docs (if accessible)
- Stable reference material
- Code examples from repositories

### Combining with codeNERD Shards

```go
// ResearcherShard integration
func (s *ResearcherShard) executeWithGrounding(ctx context.Context, task string) (*ResearchResult, error) {
    client := s.getGeminiClient()
    if client == nil {
        // Fall back to non-Gemini research
        return s.fallbackResearch(ctx, task)
    }

    // Configure URL context for documentation research
    if s.hasDocURLs() {
        client.SetURLContextURLs(s.getDocURLs())
        client.SetEnableURLContext(true)
    }

    // Enable Google Search for general research
    client.SetEnableGoogleSearch(true)

    // Execute with grounding
    resp, err := client.Complete(ctx, task)
    if err != nil {
        return nil, err
    }

    return &ResearchResult{
        Content: resp,
        Sources: client.GetLastGroundingSources(),
    }, nil
}
```

## Response Processing

### Extracting Citations

```go
func processCitations(resp *GeminiResponse) []Citation {
    gm := resp.Candidates[0].GroundingMetadata
    if gm == nil {
        return nil
    }

    var citations []Citation
    for _, support := range gm.GroundingSupports {
        for i, chunkIdx := range support.GroundingChunkIndices {
            if chunkIdx < len(gm.GroundingChunks) {
                chunk := gm.GroundingChunks[chunkIdx]
                if chunk.Web != nil {
                    citations = append(citations, Citation{
                        Text:       support.Segment.Text,
                        StartIndex: support.Segment.StartIndex,
                        EndIndex:   support.Segment.EndIndex,
                        URL:        chunk.Web.URI,
                        Title:      chunk.Web.Title,
                        Confidence: support.ConfidenceScores[i],
                    })
                }
            }
        }
    }
    return citations
}
```

### Session Context Integration

```go
// Store grounding sources for transparency
if len(resp.GroundingSources) > 0 {
    sessionCtx.GroundingSources = append(sessionCtx.GroundingSources, resp.GroundingSources...)
}
```

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| No grounding metadata | Tool not enabled | Verify `google_search` or `url_context` in tools |
| Empty sources | Query not triggering search | Rephrase for factual questions |
| URL fetch failed | URL inaccessible | Check URL accessibility, not paywalled |
| Content truncated | 34MB limit exceeded | Use smaller documents |
| Slow response | URL fetch latency | Use cached/stable URLs |
