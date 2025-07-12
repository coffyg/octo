package octo

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"io"
	"github.com/rs/zerolog"
)

func TestAllConnectionTypesWithPanics(t *testing.T) {
	// Capture logs to verify what's actually logged
	var logBuf bytes.Buffer
	testLogger := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
	oldLogger := logger
	logger = &testLogger
	defer func() { logger = oldLogger }()

	tests := []struct {
		name           string
		path           string
		headers        map[string]string
		panicValue     interface{}
		expectedLog    string
		notExpectedLog string
	}{
		{
			name: "SSE client disconnect",
			path: "/_special/rest/Sk/sse",
			headers: map[string]string{
				"Accept": "text/event-stream",
			},
			panicValue:     http.ErrAbortHandler,
			expectedLog:    "[octo-stream]",
			notExpectedLog: "[octo-panic]",
		},
		{
			name: "SSE with other accepts",
			path: "/api/events",
			headers: map[string]string{
				"Accept": "text/html, text/event-stream, application/json",
			},
			panicValue:     http.ErrAbortHandler,
			expectedLog:    "[octo-stream]",
			notExpectedLog: "[octo-panic]",
		},
		{
			name: "WebSocket disconnect",
			path: "/ws",
			headers: map[string]string{
				"Connection": "Upgrade",
				"Upgrade":    "websocket",
			},
			panicValue:     http.ErrAbortHandler,
			expectedLog:    "[octo-stream]",
			notExpectedLog: "[octo-panic]",
		},
		{
			name:           "Regular HTTP disconnect",
			path:           "/api/data",
			headers:        map[string]string{},
			panicValue:     http.ErrAbortHandler,
			expectedLog:    "[octo-panic]",
			notExpectedLog: "[octo-stream]",
		},
		{
			name: "Regular HTTP with JSON accept disconnect",
			path: "/api/users",
			headers: map[string]string{
				"Accept": "application/json",
			},
			panicValue:     http.ErrAbortHandler,
			expectedLog:    "[octo-panic]",
			notExpectedLog: "[octo-stream]",
		},
		{
			name: "SSE other panic (not disconnect)",
			path: "/_special/rest/Sk/sse",
			headers: map[string]string{
				"Accept": "text/event-stream",
			},
			panicValue:     "database error",
			expectedLog:    "[octo-panic] Panic recovered: database error",
			notExpectedLog: "[octo-stream]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			logBuf.Reset()

			// Create router
			router := NewRouter[any]()
			router.UseGlobal(RecoveryMiddleware[any]())

			// Add route that panics
			router.GET(tt.path, func(ctx *Ctx[any]) {
				// Verify connection type is set correctly
				if tt.headers["Accept"] == "text/event-stream" || strings.Contains(tt.headers["Accept"], "text/event-stream") {
					if ctx.ConnectionType != ConnectionTypeSSE {
						t.Errorf("Expected SSE connection type, got %v", ctx.ConnectionType)
					}
				} else if tt.headers["Connection"] == "Upgrade" && tt.headers["Upgrade"] == "websocket" {
					if ctx.ConnectionType != ConnectionTypeWebSocket {
						t.Errorf("Expected WebSocket connection type, got %v", ctx.ConnectionType)
					}
				} else {
					if ctx.ConnectionType != ConnectionTypeHTTP {
						t.Errorf("Expected HTTP connection type, got %v", ctx.ConnectionType)
					}
				}
				panic(tt.panicValue)
			})

			// Create request
			req := httptest.NewRequest("GET", tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Execute
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check logs
			logOutput := logBuf.String()
			t.Logf("Log output for %s: %s", tt.name, logOutput)

			if tt.expectedLog != "" && !strings.Contains(logOutput, tt.expectedLog) {
				t.Errorf("Expected log to contain %q, got: %s", tt.expectedLog, logOutput)
			}
			if tt.notExpectedLog != "" && strings.Contains(logOutput, tt.notExpectedLog) {
				t.Errorf("Expected log NOT to contain %q, got: %s", tt.notExpectedLog, logOutput)
			}
		})
	}
}

func TestSSEActualDisconnect(t *testing.T) {
	// Capture logs
	var logBuf bytes.Buffer
	testLogger := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
	oldLogger := logger
	logger = &testLogger
	defer func() { logger = oldLogger }()

	router := NewRouter[any]()
	router.UseGlobal(RecoveryMiddleware[any]())

	// SSE handler that simulates real SSE behavior
	router.GET("/events", func(ctx *Ctx[any]) {
		// Set SSE headers
		ctx.SetHeader("Content-Type", "text/event-stream")
		ctx.SetHeader("Cache-Control", "no-cache")
		ctx.SetHeader("Connection", "keep-alive")
		
		// Try to write data (this will panic if client disconnected)
		_, err := ctx.ResponseWriter.Write([]byte("data: test\n\n"))
		if err != nil {
			panic(http.ErrAbortHandler)
		}
		ctx.ResponseWriter.Flush()
	})

	// Create SSE request
	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	// Use a custom ResponseWriter that simulates disconnect
	disconnectWriter := &disconnectingResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		disconnectAt:   1, // Disconnect on first write
	}

	router.ServeHTTP(disconnectWriter, req)

	// Check logs
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "[octo-panic]") && strings.Contains(logOutput, "Client aborted request") {
		t.Errorf("SSE disconnect should not log warning, got: %s", logOutput)
	}
}

// disconnectingResponseWriter simulates a client disconnect
type disconnectingResponseWriter struct {
	http.ResponseWriter
	writeCount   int
	disconnectAt int
}

func (w *disconnectingResponseWriter) Write(b []byte) (int, error) {
	w.writeCount++
	if w.writeCount >= w.disconnectAt {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseWriter.Write(b)
}

func (w *disconnectingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}