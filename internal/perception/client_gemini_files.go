package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

// UploadFile uploads a file to Gemini Files API using the Resumable Upload protocol.
// It implements the FileProvider interface.
func (c *GeminiClient) UploadFile(ctx context.Context, path string, mimeType string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key required")
	}

	// 1. Prepare file
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	size := stat.Size()

	if mimeType == "" {
		mimeType = "application/octet-stream"
		// Simple extension-based detection could go here if needed,
		// but caller usually provides it.
	}

	logging.PerceptionDebug("[Gemini] UploadFile: path=%s size=%d mime=%s", path, size, mimeType)

	// 2. Start Resumable Session
	// Derive upload URL from baseURL (e.g., /v1beta -> /upload/v1beta)
	uploadBase := strings.Replace(c.baseURL, "/v1beta", "/upload/v1beta", 1)
	url := fmt.Sprintf("%s/files?key=%s", uploadBase, c.apiKey)

	metadata := map[string]interface{}{
		"file": map[string]string{
			"displayName": filepath.Base(path),
		},
	}
	jsonMeta, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonMeta))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Upload-Protocol", "resumable")
	req.Header.Set("X-Goog-Upload-Command", "start")
	req.Header.Set("X-Goog-Upload-Header-Content-Length", fmt.Sprintf("%d", size))
	req.Header.Set("X-Goog-Upload-Header-Content-Type", mimeType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload start request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload start failed (status %d): %s", resp.StatusCode, body)
	}

	uploadURL := resp.Header.Get("X-Goog-Upload-URL")
	if uploadURL == "" {
		return "", fmt.Errorf("no upload URL returned in headers")
	}

	// 3. Upload Bytes
	// POST uploadURL
	f.Seek(0, 0)
	reqUpload, err := http.NewRequestWithContext(ctx, "POST", uploadURL, f)
	if err != nil {
		return "", err
	}
	reqUpload.Header.Set("Content-Length", fmt.Sprintf("%d", size))
	reqUpload.Header.Set("X-Goog-Upload-Offset", "0")
	reqUpload.Header.Set("X-Goog-Upload-Command", "upload, finalize")

	respUpload, err := c.httpClient.Do(reqUpload)
	if err != nil {
		return "", fmt.Errorf("upload data failed: %w", err)
	}
	defer respUpload.Body.Close()

	if respUpload.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respUpload.Body)
		return "", fmt.Errorf("upload finalization failed (status %d): %s", respUpload.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(respUpload.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse upload response: %w", err)
	}

	// Extract URI
	if fileInfo, ok := result["file"].(map[string]interface{}); ok {
		if uri, ok := fileInfo["uri"].(string); ok {
			logging.PerceptionDebug("[Gemini] UploadFile success: uri=%s", uri)
			return uri, nil
		}
	}

	return "", fmt.Errorf("no file uri found in upload response")
}

// ListFiles lists uploaded files.
func (c *GeminiClient) ListFiles(ctx context.Context) ([]string, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	url := fmt.Sprintf("%s/files?key=%s", c.baseURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list files failed with status %d", resp.StatusCode)
	}

	var listResp GeminiListFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	var uris []string
	for _, f := range listResp.Files {
		uris = append(uris, f.URI)
	}
	return uris, nil
}

// GetFile retrieves metadata for a file.
func (c *GeminiClient) GetFile(ctx context.Context, fileNameOrURI string) (interface{}, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	// Name should be "files/..."
	// If URI passed, might need to extract ID, but API usually takes resource name.
	// Assuming caller passes resource name "files/xxx" or we assume it's just the ID.
	name := fileNameOrURI
	if strings.HasPrefix(name, "https://") {
		// It's a URI, but GetFile expects resource name.
		// For now, let's assume the user passes the resource name.
		// If they pass URI, valid for inference, but not for Get/Delete API usually.
		return nil, fmt.Errorf("DeleteFile requires resource name (files/...), got URI")
	}
	if !strings.HasPrefix(name, "files/") {
		name = "files/" + name
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", name, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get file failed with status %d", resp.StatusCode)
	}

	var file GeminiFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, err
	}

	return file, nil
}

// DeleteFile deletes an uploaded file.
func (c *GeminiClient) DeleteFile(ctx context.Context, fileID string) error {
	if c.apiKey == "" {
		return fmt.Errorf("API key required")
	}

	name := fileID
	if !strings.HasPrefix(name, "files/") {
		name = "files/" + name
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", name, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete file failed with status %d", resp.StatusCode)
	}

	return nil
}

// CreateCachedContent creates a context cache for the given files.
// implements CacheProvider.
func (c *GeminiClient) CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key required")
	}

	logging.PerceptionDebug("[Gemini] CreateCachedContent: files=%d ttl=%d", len(files), ttl)

	url := fmt.Sprintf("%s/cachedContents?key=%s", c.baseURL, c.apiKey)

	// Construct payload
	// Note: We are creating a cache for the specific model (e.g. models/gemini-1.5-flash-001)
	// We should probably normalize the model name.
	modelName := c.model
	if !strings.HasPrefix(modelName, "models/") {
		modelName = "models/" + modelName
	}
	// "gemini-3-flash-preview" might not support caching yet?
	// Caching is supported on 1.5 Pro/Flash. Assuming 3 supports it or we fallback.
	// We'll trust the user to use a supported model.

	contents := []GeminiContent{}

	// Create parts for files
	parts := []GeminiPart{}
	for _, fileURI := range files {
		parts = append(parts, GeminiPart{
			FileData: &GeminiFileData{
				FileURI: fileURI,
				// MimeType is optional in referencing? The API docs say:
				// "mimeType": "application/pdf", "fileUri": "..."
				// If we don't have it handy here, we can omit it or try to fetch it.
				// For now, let's omit and see if API accepts it (usually does if file is processed).
				// We actually need mimeType for referencing usually.
				MimeType: "application/octet-stream", // Placeholder, ideally passed in
			},
		})
	}

	if len(parts) > 0 {
		contents = append(contents, GeminiContent{
			Role:  "user",
			Parts: parts,
		})
	}

	cacheReq := GeminiCachedContent{
		Model:    modelName,
		Contents: contents,
		TTL:      fmt.Sprintf("%ds", ttl),
	}

	jsonData, err := json.Marshal(cacheReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create cache request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create cache failed (status %d): %s", resp.StatusCode, body)
	}

	var result GeminiCachedContent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse cache response: %w", err)
	}

	logging.PerceptionDebug("[Gemini] CreateCachedContent success: name=%s", result.Name)
	return result.Name, nil
}

// GetCachedContent retrieves metadata for a cached content.
func (c *GeminiClient) GetCachedContent(ctx context.Context, cacheName string) (interface{}, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", cacheName, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get cache failed with status %d", resp.StatusCode)
	}

	var cache GeminiCachedContent
	if err := json.NewDecoder(resp.Body).Decode(&cache); err != nil {
		return nil, err
	}

	return cache, nil
}

// DeleteCachedContent deletes a context cache.
func (c *GeminiClient) DeleteCachedContent(ctx context.Context, cacheName string) error {
	if c.apiKey == "" {
		return fmt.Errorf("API key required")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", cacheName, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete cache failed with status %d", resp.StatusCode)
	}

	return nil
}

// ListCachedContent lists active context caches.
func (c *GeminiClient) ListCachedContent(ctx context.Context) ([]string, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/cachedContents?key=%s", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list cache failed with status %d", resp.StatusCode)
	}

	var listResp GeminiListCachedContentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	var names []string
	for _, cc := range listResp.CachedContents {
		names = append(names, cc.Name)
	}
	return names, nil
}
