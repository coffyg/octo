package octo

import (
	"net/http/httptest"
	"testing"
)

func TestSSEPathDetection(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		shouldBeSSE bool
	}{
		// Valid SSE paths
		{"SSE endpoint", "/_special/rest/Sk/sse", true},
		{"SSE with query", "/_special/rest/Sk/sse?tabId=123", true},
		{"Simple SSE", "/sse", true},
		{"API SSE", "/api/sse", true},
		
		// Should NOT be detected as SSE
		{"JPEG file", "/assets/sse.jpeg", false},
		{"PNG file", "/images/sse.png", false},
		{"JS file", "/js/sse.js", false},
		{"CSS file", "/css/sse.css", false},
		{"HTML file", "/pages/sse.html", false},
		{"Text file", "/docs/sse.txt", false},
		{"JSON file", "/data/sse.json", false},
		{"File with sse in middle", "/assets/classes.css", false},
		{"Directory named sse", "/sse/", false},
		{"SSE in middle of path", "/api/messages/list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			
			router := NewRouter[any]()
			var detectedType ConnectionType
			
			// Add a catch-all route
			router.GET("/*path", func(ctx *Ctx[any]) {
				detectedType = ctx.ConnectionType
			})
			
			router.ServeHTTP(w, req)
			
			isSSE := detectedType == ConnectionTypeSSE
			if isSSE != tt.shouldBeSSE {
				t.Errorf("Path %s: expected SSE=%v, got SSE=%v (type=%v)", 
					tt.path, tt.shouldBeSSE, isSSE, detectedType)
			}
		})
	}
}