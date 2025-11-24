package example

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestSimpleUpload(t *testing.T) {
	if os.Getenv("INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=1 to run")
	}

	// Simple content
	content := "This is a simple test document for testing upload functionality."

	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "simple_test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.WriteString(part, content)
	if err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}

	writer.Close()

	// Make upload request
	req, err := http.NewRequest("POST", "http://localhost:8081/api/v1/documents/upload", &body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Use shorter timeout to avoid hanging
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	if resp == nil {
		t.Fatalf("Upload response is nil")
	}
	defer resp.Body.Close()

	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response headers: %v", resp.Header)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	t.Logf("Response body: %s", string(respBody))

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}
