package octo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/rs/zerolog"
)

// Custom data type for Ctx.Custom
type CustomData struct {
	UserID string
}

// Simple handler for testing
func testHandler(ctx *Ctx[CustomData]) {
	ctx.SendJSON(http.StatusOK, map[string]string{
		"message": "handler executed",
		"userID":  ctx.Custom.UserID,
	})
}

// Middleware that adds a value to CustomData
func customMiddleware(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
	return func(ctx *Ctx[CustomData]) {
		ctx.Custom.UserID = "middleware_user"
		next(ctx)
	}
}

// Middleware that modifies CustomData
func appendMiddleware[V any](next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
	return func(ctx *Ctx[CustomData]) {
		// change middleware_user to middleware_user_modified
		ctx.Custom.UserID = ctx.Custom.UserID + "_modified"
		next(ctx)
	}
}

func TestRouter(t *testing.T) {
	router := NewRouter[CustomData]()

	// Add routes
	router.GET("/static", testHandler)
	router.GET("/param/:value", testHandler)
	router.POST("/static", testHandler)

	// Test static route
	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	expectedBody := `{"message":"handler executed","userID":""}`
	var expectedData map[string]interface{}
	var actualData map[string]interface{}

	// Unmarshal expected and actual JSON
	if err := json.Unmarshal([]byte(expectedBody), &expectedData); err != nil {
		t.Fatalf("Failed to unmarshal expected body: %v", err)
	}
	if err := json.Unmarshal(bodyBytes, &actualData); err != nil {
		t.Fatalf("Failed to unmarshal actual body: %v", err)
	}

	// Compare the data
	if !reflect.DeepEqual(expectedData, actualData) {
		t.Errorf("Expected body %v, got %v", expectedData, actualData)
	}

	// Test parameterized route
	req = httptest.NewRequest("GET", "/param/testvalue", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test undefined route
	req = httptest.NewRequest("GET", "/undefined", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestMiddleware(t *testing.T) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	SetupOctoLogger(&logger)
	router := NewRouter[CustomData]()

	// Add global middleware
	router.Use(customMiddleware)

	// Add route-specific middleware
	router.GET("/middleware", testHandler, appendMiddleware[CustomData])

	// Test middleware effect
	req := httptest.NewRequest("GET", "/middleware", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check if CustomData was modified by middleware
	expectedBody := `{"message":"handler executed","userID":"middleware_user_modified"}`
	var actualData map[string]interface{}
	var expectedData map[string]interface{}

	// Unmarshal expected and actual JSON
	if err := json.Unmarshal([]byte(expectedBody), &expectedData); err != nil {
		t.Fatalf("Failed to unmarshal expected body: %v", err)
	}
	if err := json.Unmarshal(body, &actualData); err != nil {
		t.Fatalf("Failed to unmarshal actual body: %v", err)
	}

	// Compare the data
	if !reflect.DeepEqual(expectedData, actualData) {
		t.Errorf("Expected body %v, got %v", expectedData, actualData)
	}

	// Ensure middleware executed in correct order
}

func TestHTTPMethods(t *testing.T) {
	router := NewRouter[CustomData]()

	// Add handlers for different methods
	router.GET("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("GET method"))
	})
	router.POST("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("POST method"))
	})

	// Test GET method
	req := httptest.NewRequest("GET", "/method", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != "GET method" {
		t.Errorf("Expected 'GET method', got '%s'", string(body))
	}

	// Test POST method
	req = httptest.NewRequest("POST", "/method", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)

	if string(body) != "POST method" {
		t.Errorf("Expected 'POST method', got '%s'", string(body))
	}

	// Test undefined method
	req = httptest.NewRequest("PUT", "/method", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for undefined method, got %d", resp.StatusCode)
	}
}

func TestRouteGroups(t *testing.T) {
	router := NewRouter[CustomData]()

	// Middleware to test group application
	groupMiddlewareCalled := false
	groupMiddleware := func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			groupMiddlewareCalled = true
			next(ctx)
		}
	}

	// Create a group with middleware
	apiGroup := router.Group("/api", groupMiddleware)

	// Add a route to the group
	apiGroup.GET("/test", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Test that middleware is called
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if !groupMiddlewareCalled {
		t.Errorf("Expected group middleware to be called")
	}

	// Reset for next test
	groupMiddlewareCalled = false

	// Add a route outside the group
	router.GET("/test", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Test that middleware is not called for routes outside the group
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if groupMiddlewareCalled {
		t.Errorf("Expected group middleware not to be called")
	}
}

func TestWildcardRoute(t *testing.T) {
	router := NewRouter[CustomData]()

	// Handler for wildcard route
	router.GET("/files/*filepath", func(ctx *Ctx[CustomData]) {
		filepath := ctx.Params["filepath"]
		ctx.ResponseWriter.Write([]byte("Filepath: " + filepath))
	})

	// Test wildcard route
	req := httptest.NewRequest("GET", "/files/images/logo.png", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	expectedBody := "Filepath: images/logo.png"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestComplexParameterRoute(t *testing.T) {
	router := NewRouter[CustomData]()

	// Handler for complex parameter route
	router.GET("/public/Thread/:uuid/v/:version", func(ctx *Ctx[CustomData]) {
		uuid := ctx.Params["uuid"]
		version := ctx.Params["version"]
		ctx.ResponseWriter.Write([]byte("UUID: " + uuid + ", Version: " + version))
	})

	// Test complex parameter route
	req := httptest.NewRequest("GET", "/public/Thread/123e4567-e89b-12d3-a456-426614174000/v/2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	expectedBody := "UUID: 123e4567-e89b-12d3-a456-426614174000, Version: 2"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}
func TestEmbeddedParameterRoute(t *testing.T) {
	router := NewRouter[CustomData]()

	// Handler for embedded parameter route
	router.GET("/User:action", func(ctx *Ctx[CustomData]) {
		action := ctx.Param("action")
		ctx.ResponseWriter.Write([]byte("Action: " + action))
	})

	// Test embedded parameter route
	req := httptest.NewRequest("GET", "/User:get", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	expectedBody := "Action: :get"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestUseGlobalMiddlewareWithNoRoute(t *testing.T) {
	router := NewRouter[CustomData]()

	// Variable to check if middleware is called
	middlewareCalled := false

	// Global middleware
	router.UseGlobal(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			middlewareCalled = true
			ctx.ResponseWriter.Write([]byte("Middleware executed"))
			ctx.Done() // Stop further processing
			next(ctx)
		}
	})

	// Do not add any routes

	// Send request to undefined route
	req := httptest.NewRequest("GET", "/undefined", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if !middlewareCalled {
		t.Errorf("Expected global middleware to be called")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "Middleware executed" {
		t.Errorf("Expected response 'Middleware executed', got '%s'", string(body))
	}
}

func TestUseGlobalMiddlewareWithoutDone(t *testing.T) {
	router := NewRouter[CustomData]()

	// Variable to check if middleware is called
	middlewareCalled := false

	// Global middleware
	router.UseGlobal(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			middlewareCalled = true
			next(ctx)
		}
	})

	// Do not add any routes

	// Send request to undefined route
	req := httptest.NewRequest("GET", "/undefined", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()

	if !middlewareCalled {
		t.Errorf("Expected global middleware to be called")
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}
func TestRouter_PanicRecovery(t *testing.T) {
	router := NewRouter[CustomData]()

	// Add Recovery Middleware
	router.UseGlobal(RecoveryMiddleware[CustomData]())

	// Define a route that panics
	router.GET("/panic", func(ctx *Ctx[CustomData]) {
		panic("test panic")
	})

	// Send a request to the panic route
	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Optionally, check the response body for the standardized error message
	// For example, expecting JSON with "result": "error" and "message": "Internal error"
	// You can decode the JSON and assert the fields accordingly
}
func TestSecurityHeaders(t *testing.T) {
	// Save the old setting
	oldVal := EnableSecurityHeaders

	// Create a new router
	router := NewRouter[CustomData]()

	// Basic route
	router.GET("/test", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Hello"))
	})

	// 1) Security headers DISABLED
	EnableSecurityHeaders = false
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("X-Content-Type-Options") != "" {
		t.Errorf("Expected no X-Content-Type-Options when security headers disabled")
	}

	// 2) Security headers ENABLED
	EnableSecurityHeaders = true
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp = w.Result()

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected 'nosniff', got '%s'", resp.Header.Get("X-Content-Type-Options"))
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected 'DENY', got '%s'", resp.Header.Get("X-Frame-Options"))
	}
	if resp.Header.Get("X-XSS-Protection") != "1; mode=block" {
		t.Errorf("Expected '1; mode=block', got '%s'", resp.Header.Get("X-XSS-Protection"))
	}

	// Restore
	EnableSecurityHeaders = oldVal
}
func TestDeferBufferAllocation(t *testing.T) {
	oldVal := DeferBufferAllocation
	defer func() { DeferBufferAllocation = oldVal }()

	// If DeferBufferAllocation = false, we expect a non-nil Body in the wrapper immediately
	DeferBufferAllocation = false
	rw := NewResponseWriterWrapper(httptest.NewRecorder())
	if rw.Body == nil {
		t.Errorf("Expected Body to be allocated immediately when DeferBufferAllocation=false")
	}

	// If DeferBufferAllocation = true, we expect a nil Body initially
	DeferBufferAllocation = true
	rw = NewResponseWriterWrapper(httptest.NewRecorder())
	if rw.Body != nil {
		t.Errorf("Expected Body to be nil initially when DeferBufferAllocation=true")
	}
}
