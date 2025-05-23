package octo

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFileServing(t *testing.T) {
	// Create a test directory with files
	tempDir := t.TempDir()
	
	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Hello, World!"), 0644); err != nil {
		t.Fatal(err)
	}
	
	indexFile := filepath.Join(tempDir, "index.html")
	if err := os.WriteFile(indexFile, []byte("<h1>Index</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create subdirectory
	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	subFile := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(subFile, []byte("Subfile"), 0644); err != nil {
		t.Fatal(err)
	}
	
	router := NewRouter[any]()
	
	// Mount static handler - handle both with and without trailing slash
	staticHandler := Static[any]("/static/", StaticConfig{
		Root:          tempDir,
		EnableCaching: true,
		MaxAge:        3600,
	})
	router.GET("/static/*filepath", staticHandler)
	
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
		expectedType   string
	}{
		{
			name:           "serve text file",
			path:           "/static/test.txt",
			expectedStatus: 200,
			expectedBody:   "Hello, World!",
			expectedType:   "application/octet-stream",
		},
		{
			name:           "serve index for root without slash",
			path:           "/static/index.html",
			expectedStatus: 200,
			expectedBody:   "<h1>Index</h1>",
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "serve subdirectory file",
			path:           "/static/sub/file.txt",
			expectedStatus: 200,
			expectedBody:   "Subfile",
			expectedType:   "application/octet-stream",
		},
		{
			name:           "404 for non-existent file",
			path:           "/static/notfound.txt",
			expectedStatus: 404,
		},
		{
			name:           "prevent directory traversal",
			path:           "/static/../etc/passwd",
			expectedStatus: 400,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				if string(body) != tt.expectedBody {
					t.Errorf("Expected body '%s', got '%s'", tt.expectedBody, string(body))
				}
			}
			
			if tt.expectedType != "" {
				contentType := resp.Header.Get("Content-Type")
				if contentType != tt.expectedType {
					t.Errorf("Expected content-type '%s', got '%s'", tt.expectedType, contentType)
				}
			}
		})
	}
}

func TestStaticCaching(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "cached.txt")
	if err := os.WriteFile(testFile, []byte("Cached content"), 0644); err != nil {
		t.Fatal(err)
	}
	
	router := NewRouter[any]()
	router.GET("/static/*filepath", Static[any]("/static/", StaticConfig{
		Root:          tempDir,
		EnableCaching: true,
		MaxAge:        3600,
	}))
	
	// First request
	req1 := httptest.NewRequest("GET", "/static/cached.txt", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	
	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header")
	}
	
	// Second request with If-None-Match
	req2 := httptest.NewRequest("GET", "/static/cached.txt", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	
	if w2.Code != 304 {
		t.Errorf("Expected 304 Not Modified, got %d", w2.Code)
	}
}