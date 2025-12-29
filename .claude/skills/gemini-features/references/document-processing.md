# Document Processing Reference

## Overview

Gemini can process various document types including PDFs, images, and text files. Documents can be sent inline (base64) or via URL.

## Supported Formats

| Format | MIME Type | Max Size | Notes |
|--------|-----------|----------|-------|
| PDF | `application/pdf` | 20 MB | Up to 3,600 pages |
| Images | `image/jpeg`, `image/png`, `image/webp`, `image/gif` | 20 MB | Multiple images supported |
| Text | `text/plain` | 20 MB | Plain text files |
| HTML | `text/html` | 20 MB | Web pages |
| Markdown | `text/md` | 20 MB | Markdown files |
| Code | Various | 20 MB | Source code files |

## Inline Document Upload (Base64)

### Request Structure

```go
type GeminiInlineData struct {
    MimeType string `json:"mimeType"`
    Data     string `json:"data"` // Base64 encoded
}

type GeminiPart struct {
    Text       string            `json:"text,omitempty"`
    InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}
```

### PDF Processing Example

```go
func (c *GeminiClient) ProcessPDF(ctx context.Context, pdfPath string, prompt string) (string, error) {
    // Read and encode PDF
    pdfData, err := os.ReadFile(pdfPath)
    if err != nil {
        return "", fmt.Errorf("failed to read PDF: %w", err)
    }

    // Build request with inline PDF
    content := GeminiContent{
        Role: "user",
        Parts: []GeminiPart{
            {
                InlineData: &GeminiInlineData{
                    MimeType: "application/pdf",
                    Data:     base64.StdEncoding.EncodeToString(pdfData),
                },
            },
            {
                Text: prompt,
            },
        },
    }

    return c.complete(ctx, content)
}
```

### REST API Format

```bash
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{
      "parts": [
        {
          "inlineData": {
            "mimeType": "application/pdf",
            "data": "<base64_encoded_pdf>"
          }
        },
        {"text": "Summarize this document"}
      ]
    }]
  }'
```

## URL-Based Documents

For larger files, use Google Cloud Storage URLs:

```go
type GeminiFileData struct {
    MimeType string `json:"mimeType"`
    FileURI  string `json:"fileUri"` // gs:// URL
}

type GeminiPart struct {
    Text     string           `json:"text,omitempty"`
    FileData *GeminiFileData  `json:"fileData,omitempty"`
}
```

### Google Cloud Storage Upload

```go
func uploadToGCS(ctx context.Context, data []byte, bucket, object string) (string, error) {
    client, err := storage.NewClient(ctx)
    if err != nil {
        return "", err
    }
    defer client.Close()

    wc := client.Bucket(bucket).Object(object).NewWriter(ctx)
    if _, err := wc.Write(data); err != nil {
        return "", err
    }
    if err := wc.Close(); err != nil {
        return "", err
    }

    return fmt.Sprintf("gs://%s/%s", bucket, object), nil
}
```

## Multiple Documents

Process multiple documents in a single request:

```go
func (c *GeminiClient) ProcessMultipleDocuments(
    ctx context.Context,
    documents []Document,
    prompt string,
) (string, error) {
    var parts []GeminiPart

    for _, doc := range documents {
        parts = append(parts, GeminiPart{
            InlineData: &GeminiInlineData{
                MimeType: doc.MimeType,
                Data:     base64.StdEncoding.EncodeToString(doc.Data),
            },
        })
    }

    // Add prompt at the end
    parts = append(parts, GeminiPart{Text: prompt})

    content := GeminiContent{
        Role:  "user",
        Parts: parts,
    }

    return c.complete(ctx, content)
}
```

## Image Processing

### Single Image

```go
func (c *GeminiClient) AnalyzeImage(ctx context.Context, imagePath string, prompt string) (string, error) {
    imageData, err := os.ReadFile(imagePath)
    if err != nil {
        return "", err
    }

    mimeType := detectMimeType(imagePath)

    content := GeminiContent{
        Role: "user",
        Parts: []GeminiPart{
            {
                InlineData: &GeminiInlineData{
                    MimeType: mimeType,
                    Data:     base64.StdEncoding.EncodeToString(imageData),
                },
            },
            {Text: prompt},
        },
    }

    return c.complete(ctx, content)
}

func detectMimeType(path string) string {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".jpg", ".jpeg":
        return "image/jpeg"
    case ".png":
        return "image/png"
    case ".gif":
        return "image/gif"
    case ".webp":
        return "image/webp"
    default:
        return "application/octet-stream"
    }
}
```

### Multiple Images (Comparison)

```go
func (c *GeminiClient) CompareImages(
    ctx context.Context,
    images []string,
    prompt string,
) (string, error) {
    var parts []GeminiPart

    for _, imagePath := range images {
        imageData, err := os.ReadFile(imagePath)
        if err != nil {
            return "", err
        }

        parts = append(parts, GeminiPart{
            InlineData: &GeminiInlineData{
                MimeType: detectMimeType(imagePath),
                Data:     base64.StdEncoding.EncodeToString(imageData),
            },
        })
    }

    parts = append(parts, GeminiPart{Text: prompt})

    content := GeminiContent{
        Role:  "user",
        Parts: parts,
    }

    return c.complete(ctx, content)
}
```

## PDF-Specific Features

### Page Extraction

Gemini can reference specific pages:

```
"Summarize page 5 of the document"
"Compare figures on pages 10 and 15"
"Extract the table from page 3"
```

### OCR Capabilities

- Scanned documents are automatically OCR'd
- Handwriting recognition supported
- Tables and charts are analyzed

### Large PDF Handling

For PDFs > 20 MB:
1. Upload to Google Cloud Storage
2. Use `fileUri` instead of `inlineData`
3. Consider splitting into chunks

```go
func (c *GeminiClient) ProcessLargePDF(ctx context.Context, gcsURI string, prompt string) (string, error) {
    content := GeminiContent{
        Role: "user",
        Parts: []GeminiPart{
            {
                FileData: &GeminiFileData{
                    MimeType: "application/pdf",
                    FileURI:  gcsURI,
                },
            },
            {Text: prompt},
        },
    }

    return c.complete(ctx, content)
}
```

## codeNERD Integration

### Document Ingestion for Campaigns

```go
// internal/campaign/document_ingestor.go
func (i *DocumentIngestor) IngestPDF(ctx context.Context, pdfPath string) (*IngestResult, error) {
    client := i.getGeminiClient()
    if client == nil {
        return nil, fmt.Errorf("document ingestion requires Gemini provider")
    }

    // Extract structured information
    extractPrompt := `Analyze this document and extract:
1. Main topics and sections
2. Key concepts and definitions
3. Code examples (if any)
4. Action items or requirements

Format as JSON with keys: topics, concepts, codeExamples, actionItems`

    result, err := client.ProcessPDF(ctx, pdfPath, extractPrompt)
    if err != nil {
        return nil, err
    }

    return &IngestResult{
        Content:    result,
        SourcePath: pdfPath,
        Type:       "pdf",
    }, nil
}
```

### ResearcherShard Document Analysis

```go
func (s *ResearcherShard) analyzeDocumentation(ctx context.Context, docs []string) (*AnalysisResult, error) {
    client := s.getGeminiClient()

    var documents []Document
    for _, docPath := range docs {
        data, _ := os.ReadFile(docPath)
        documents = append(documents, Document{
            Path:     docPath,
            Data:     data,
            MimeType: detectMimeType(docPath),
        })
    }

    result, err := client.ProcessMultipleDocuments(ctx, documents,
        "Analyze these documents and identify key APIs, patterns, and usage examples")

    return &AnalysisResult{Content: result}, err
}
```

## Token Counting

Documents count toward input tokens:

| Document Type | Token Estimation |
|--------------|------------------|
| PDF | ~250 tokens per page (text-heavy) |
| Image | ~258 tokens per image |
| Code file | ~4 tokens per line |

### Checking Token Usage

```go
func (c *GeminiClient) EstimateDocumentTokens(data []byte, mimeType string) int {
    switch {
    case strings.HasPrefix(mimeType, "image/"):
        return 258
    case mimeType == "application/pdf":
        // Rough estimate: 1 page ≈ 3KB, 250 tokens/page
        pages := len(data) / 3000
        return pages * 250
    default:
        // Text: ~4 chars per token
        return len(data) / 4
    }
}
```

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| 400: Invalid document | Corrupted or unsupported format | Verify file integrity and format |
| 400: Document too large | Exceeds 20 MB inline limit | Use GCS fileUri instead |
| 400: Too many pages | Exceeds 3,600 page limit | Split PDF into chunks |
| 413: Request too large | Total request exceeds limit | Reduce document count/size |

## Best Practices

1. **Compress before sending** - Reduce image quality for faster processing
2. **Use GCS for large files** - Avoid base64 overhead for files > 5 MB
3. **Batch related documents** - Process related docs together for context
4. **Verify mime types** - Ensure correct mime type for each document
5. **Handle failures gracefully** - Some pages may fail OCR; handle partial results
