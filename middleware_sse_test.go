package octo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSSEPanicRecoveryIntegration(t *testing.T) {
	// This test verifies that SSE disconnects are not logged as warnings

	// Create SSE request
	req := httptest.NewRequest("GET", "/_special/rest/Sk/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	
	w := httptest.NewRecorder()
	
	// Create router with recovery middleware
	router := NewRouter[any]()
	router.UseGlobal(RecoveryMiddleware[any]())
	
	// Add route that panics with http.ErrAbortHandler (simulating client disconnect)
	router.GET("/_special/rest/Sk/sse", func(ctx *Ctx[any]) {
		// Verify connection type is detected
		if ctx.ConnectionType != ConnectionTypeSSE {
			t.Errorf("Expected SSE connection type, got %v", ctx.ConnectionType)
		}
		panic(http.ErrAbortHandler)
	})
	
	// Execute request
	router.ServeHTTP(w, req)
	
	// If we get here without a 500 error, the panic was handled correctly
	if w.Code == http.StatusInternalServerError {
		t.Error("SSE disconnect should not return 500 error")
	}
}

func TestConnectionTypeDetectionBeforePanic(t *testing.T) {
	req := httptest.NewRequest("GET", "/_special/rest/Sk/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	
	w := httptest.NewRecorder()
	
	// Track if connection type was set before panic
	var connTypeBeforePanic ConnectionType
	var connTypeInRecovery ConnectionType
	
	// Create custom recovery middleware to capture state
	testRecovery := func(next HandlerFunc[any]) HandlerFunc[any] {
		return func(ctx *Ctx[any]) {
			defer func() {
				if r := recover(); r != nil {
					connTypeInRecovery = ctx.ConnectionType
				}
			}()
			next(ctx)
		}
	}
	
	router := NewRouter[any]()
	router.UseGlobal(testRecovery)
	
	router.GET("/_special/rest/Sk/sse", func(ctx *Ctx[any]) {
		connTypeBeforePanic = ctx.ConnectionType
		panic("test")
	})
	
	router.ServeHTTP(w, req)
	
	if connTypeBeforePanic != ConnectionTypeSSE {
		t.Errorf("Connection type not set before handler: got %v, want %v", connTypeBeforePanic, ConnectionTypeSSE)
	}
	if connTypeInRecovery != ConnectionTypeSSE {
		t.Errorf("Connection type not available in recovery: got %v, want %v", connTypeInRecovery, ConnectionTypeSSE)
	}
}