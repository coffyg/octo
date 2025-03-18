package octo

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestErrorContextMiddleware(t *testing.T) {
    // Create a new router
    router := NewRouter[string]()
    
    // Add the error context middleware
    router.Use(ErrorContextMiddleware[string]())
    
    // Add a handler that checks for context
    router.GET("/test-error-context", func(ctx *Ctx[string]) {
        // Get request info from context
        info, ok := ctx.Request.Context().Value(RequestErrorKey{}).(*RequestErrorInfo)
        if !ok {
            t.Error("Request info not found in context")
            ctx.SendError(string(ErrInternal), New(ErrInternal, "Context not found"))
            return
        }
        
        // Verify the information
        if info.Method != "GET" {
            t.Errorf("Expected method GET, got %s", info.Method)
        }
        
        if info.Path != "/test-error-context" {
            t.Errorf("Expected path /test-error-context, got %s", info.Path)
        }
        
        // Return success
        ctx.SendString(http.StatusOK, "OK")
    })
    
    // Create a test request
    req := httptest.NewRequest("GET", "/test-error-context", nil)
    req.Header.Set("User-Agent", "Go-Test-Client")
    
    // Create a response recorder
    w := httptest.NewRecorder()
    
    // Serve the request
    router.ServeHTTP(w, req)
    
    // Check the response
    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
}

func TestEnhanceLogErrorsMiddleware(t *testing.T) {
    // Create a new router
    router := NewRouter[string]()
    
    // Add both middlewares
    router.Use(ErrorContextMiddleware[string]())
    router.Use(EnhanceLogErrorsMiddleware[string]())
    
    // Add a handler that returns an error
    router.GET("/test-error-logging", func(ctx *Ctx[string]) {
        ctx.SendError(string(ErrInvalidRequest), New(ErrInvalidRequest, "Test error"))
    })
    
    // Create a test request
    req := httptest.NewRequest("GET", "/test-error-logging", nil)
    req.Header.Set("User-Agent", "Go-Test-Client")
    req.Header.Set("X-Request-ID", "test-123")
    
    // Create a response recorder
    w := httptest.NewRecorder()
    
    // Serve the request
    router.ServeHTTP(w, req)
    
    // Check the response
    if w.Code != http.StatusBadRequest {
        t.Errorf("Expected status 400, got %d", w.Code)
    }
}