package octo

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/rs/zerolog"
)

func TestFWSSpecialRoutes(t *testing.T) {
	// Capture logs
	var logBuf bytes.Buffer
	testLogger := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
	oldLogger := logger
	logger = &testLogger
	defer func() { logger = oldLogger }()

	// Test the exact paths you're seeing in logs
	paths := []string{
		"/_special/rest/Sk/sse",
		"/_special/rest/DevSk/sse",
		"/_special/rest/Realm/123/sse",
		"/_special/rest/Public/sse",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			logBuf.Reset()

			router := NewRouter[any]()
			router.UseGlobal(RecoveryMiddleware[any]())

			// Add route
			router.GET(path, func(ctx *Ctx[any]) {
				// Verify SSE was detected
				if ctx.ConnectionType != ConnectionTypeSSE {
					t.Errorf("Path %s: Expected SSE connection type, got %v", path, ctx.ConnectionType)
				}
				// Simulate disconnect
				panic(http.ErrAbortHandler)
			})

			// Create SSE request
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("Accept", "text/event-stream")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify logs
			logOutput := logBuf.String()
			t.Logf("Logs for %s: %s", path, logOutput)

			// Should NOT have warning level panic log
			if strings.Contains(logOutput, `"level":"warn"`) && strings.Contains(logOutput, "[octo-panic]") {
				t.Errorf("Path %s: Should not log SSE disconnect as warning", path)
			}

			// Should have debug level stream log
			if !strings.Contains(logOutput, `"level":"debug"`) || !strings.Contains(logOutput, "[octo-stream]") {
				t.Errorf("Path %s: Should log SSE disconnect at debug level", path)
			}
		})
	}
}