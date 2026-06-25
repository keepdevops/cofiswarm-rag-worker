// Package ingest posts a file to the cofiswarm-rag service's multipart /ingest endpoint.
// The rag service chunks + embeds asynchronously (returns a job id), so a 2xx here means the
// file was accepted for indexing, not that embedding has finished.
package ingest

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 60 * time.Second}

// Post uploads filePath to <ragURL>/ingest as multipart field "file".
func Post(ragURL, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return fmt.Errorf("copy %s: %w", filePath, err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("multipart close: %w", err)
	}

	url := strings.TrimRight(ragURL, "/") + "/ingest"
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ingest %s: %s: %s", url, resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}
