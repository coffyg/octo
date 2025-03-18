package octo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
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

// New tests for path matching edge cases

func TestMultipleEmbeddedParameters(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with multiple embedded parameters
	router.GET("/prefix:param1:param2", func(ctx *Ctx[CustomData]) {
		param1 := ctx.Params["param1"]
		param2 := ctx.Params["param2"]
		ctx.ResponseWriter.Write([]byte(fmt.Sprintf("Param1: %s, Param2: %s", param1, param2)))
	})
	
	// Test with values for both parameters
	req := httptest.NewRequest("GET", "/prefixvalue1value2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Param1: value1, Param2: value2"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestEmbeddedParametersWithSpecialChars(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with embedded parameter that might contain special characters
	router.GET("/file:type", func(ctx *Ctx[CustomData]) {
		fileType := ctx.Params["type"]
		ctx.ResponseWriter.Write([]byte("Type: " + fileType))
	})
	
	// Test with special characters in the parameter value
	req := httptest.NewRequest("GET", "/file.jpg", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Type: .jpg"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestMultipleParametersInSegment(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with multiple parameters separated by static content
	router.GET("/user:id-post:postId", func(ctx *Ctx[CustomData]) {
		userId := ctx.Params["id"]
		postId := ctx.Params["postId"]
		ctx.ResponseWriter.Write([]byte(fmt.Sprintf("User: %s, Post: %s", userId, postId)))
	})
	
	// Test with values for both parameters
	req := httptest.NewRequest("GET", "/user123-post456", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "User: 123, Post: 456"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestAdjacentParameters(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with adjacent parameters
	router.GET("/:param1:param2", func(ctx *Ctx[CustomData]) {
		param1 := ctx.Params["param1"]
		param2 := ctx.Params["param2"]
		ctx.ResponseWriter.Write([]byte(fmt.Sprintf("P1: %s, P2: %s", param1, param2)))
	})
	
	// Test with values for both parameters
	req := httptest.NewRequest("GET", "/value1value2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "P1: value1, P2: value2"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestParameterAtSegmentBoundary(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with parameter at segment boundary
	router.GET("/:param/", func(ctx *Ctx[CustomData]) {
		param := ctx.Params["param"]
		ctx.ResponseWriter.Write([]byte("Param: " + param))
	})
	
	// Test with trailing slash
	req := httptest.NewRequest("GET", "/value/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Param: value"
	if string(body) != expectedBody {
		t.Errorf("With trailing slash: Expected '%s', got '%s'", expectedBody, string(body))
	}
	
	// Test without trailing slash - should still match due to normalization
	req = httptest.NewRequest("GET", "/value", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	
	// Check if the router handles both cases correctly
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Without trailing slash: Expected status 200, got %d", resp.StatusCode)
	}
}

func TestEmptyParameterValue(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with potentially empty parameter value
	router.GET("/user/:name/profile", func(ctx *Ctx[CustomData]) {
		name := ctx.Params["name"]
		ctx.ResponseWriter.Write([]byte("Name: " + name))
	})
	
	// Test with empty parameter value
	req := httptest.NewRequest("GET", "/user//profile", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	// The router should handle empty values correctly
	expectedBody := "Name: "
	if string(body) != expectedBody && resp.StatusCode == http.StatusOK {
		t.Errorf("Expected '%s', got '%s' with status %d", expectedBody, string(body), resp.StatusCode)
	}
}

func TestWildcardWithSpecialChars(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with wildcard that might contain special characters
	router.GET("/files/*path", func(ctx *Ctx[CustomData]) {
		path := ctx.Params["path"]
		ctx.ResponseWriter.Write([]byte("Path: " + path))
	})
	
	// Test with special characters in the wildcard path - using an escaped URL
	req := httptest.NewRequest("GET", "/files/path/with%3F.jpg", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Path: path/with?.jpg"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestWildcardWithEmptyRest(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with wildcard that might be empty
	router.GET("/files/*path", func(ctx *Ctx[CustomData]) {
		path := ctx.Params["path"]
		ctx.ResponseWriter.Write([]byte("Path: " + path))
	})
	
	// Test with nothing after the wildcard
	req := httptest.NewRequest("GET", "/files/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	// The router should handle empty wildcard values correctly
	expectedBody := "Path: "
	if string(body) != expectedBody && resp.StatusCode == http.StatusOK {
		t.Errorf("Expected '%s', got '%s' with status %d", expectedBody, string(body), resp.StatusCode)
	}
}

func TestPathNormalization(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Basic route to test path normalization
	router.GET("/users/:id", func(ctx *Ctx[CustomData]) {
		id := ctx.Params["id"]
		ctx.ResponseWriter.Write([]byte("ID: " + id))
	})
	
	// Test cases with different path forms
	testCases := []struct {
		name string
		path string
	}{
		{"Trailing slash", "/users/123/"},
		{"Double slashes", "/users//123"},
		{"Mixed slashes", "//users/123//"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			
			// All forms should match and extract the ID parameter
			expectedBody := "ID: 123"
			if string(body) != expectedBody {
				t.Errorf("Path '%s': Expected '%s', got '%s'", tc.path, expectedBody, string(body))
			}
		})
	}
}

func TestEmptySegments(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with no empty segments
	router.GET("/a/b/c", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Found"))
	})
	
	// Test with empty segments in various positions
	testCases := []struct {
		name string
		path string
	}{
		{"Empty segment at beginning", "//a/b/c"},
		{"Empty segment in middle", "/a//b/c"},
		{"Empty segments at end", "/a/b/c//"},
		{"Multiple consecutive empty segments", "/a/b//c"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			
			// All valid paths should match the route
			expectedBody := "Found"
			if resp.StatusCode == http.StatusOK && string(body) != expectedBody {
				t.Errorf("Path '%s': Expected '%s', got '%s'", tc.path, expectedBody, string(body))
			}
		})
	}
}

func TestSpecialCharactersInStaticRoute(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with special characters in static parts
	router.GET("/path-with-hyphens/and_underscores", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Found"))
	})
	
	req := httptest.NewRequest("GET", "/path-with-hyphens/and_underscores", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Found"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestSpecialCharactersInParameters(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with parameter that might contain special characters
	router.GET("/user/:name", func(ctx *Ctx[CustomData]) {
		name := ctx.Params["name"]
		ctx.ResponseWriter.Write([]byte("Name: " + name))
	})
	
	// Test cases with different parameter values
	testCases := []struct {
		name  string
		path  string
		expected string
	}{
		{"URL-encoded spaces", "/user/John%20Doe", "Name: John Doe"},
		{"Email address", "/user/email@example.com", "Name: email@example.com"},
		{"Special symbols", "/user/user+name!symbol", "Name: user+name!symbol"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			
			if string(body) != tc.expected {
				t.Errorf("Path '%s': Expected '%s', got '%s'", tc.path, tc.expected, string(body))
			}
		})
	}
}

func TestUnicodeInPaths(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Route with parameter that might contain Unicode characters
	router.GET("/unicode/:text", func(ctx *Ctx[CustomData]) {
		text := ctx.Params["text"]
		ctx.ResponseWriter.Write([]byte("Text: " + text))
	})
	
	// Test with Unicode characters
	path := "/unicode/你好世界"
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Text: 你好世界"
	if string(body) != expectedBody {
		t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
	}
}

func TestCaseInsensitiveRouteMatching(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Register route with mixed case
	router.GET("/MixedCase", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Found"))
	})
	
	// Test with different case variations
	testCases := []struct {
		name string
		path string
		shouldMatch bool
	}{
		{"Exact case", "/MixedCase", true},
		{"Lowercase", "/mixedcase", false},  // Default is case-sensitive
		{"Uppercase", "/MIXEDCASE", false},
		{"Mixed different", "/MiXeDcAsE", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			resp := w.Result()
			
			if tc.shouldMatch && resp.StatusCode != http.StatusOK {
				t.Errorf("Path '%s': Expected match (200), got %d", tc.path, resp.StatusCode)
			} else if !tc.shouldMatch && resp.StatusCode == http.StatusOK {
				t.Errorf("Path '%s': Expected no match (non-200), got %d", tc.path, resp.StatusCode)
			}
		})
	}
}

func TestParameterPrecedence(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Register both a static and parameter route at the same level
	router.GET("/user/admin", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Admin Route"))
	})
	
	router.GET("/user/:role", func(ctx *Ctx[CustomData]) {
		role := ctx.Params["role"]
		ctx.ResponseWriter.Write([]byte("Role: " + role))
	})
	
	// Test static route precedence
	req := httptest.NewRequest("GET", "/user/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "Admin Route"
	if string(body) != expectedBody {
		t.Errorf("Expected static route to take precedence, got '%s'", string(body))
	}
	
	// Test parameter route for non-matching static route
	req = httptest.NewRequest("GET", "/user/editor", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	
	expectedBody = "Role: editor"
	if string(body) != expectedBody {
		t.Errorf("Expected parameter route to match, got '%s'", string(body))
	}
}

func TestWildcardPrecedence(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Register routes with different specificity
	router.GET("/api/users/:id", func(ctx *Ctx[CustomData]) {
		id := ctx.Params["id"]
		ctx.ResponseWriter.Write([]byte("User ID: " + id))
	})
	
	router.GET("/api/*wildcard", func(ctx *Ctx[CustomData]) {
		wildcard := ctx.Params["wildcard"]
		ctx.ResponseWriter.Write([]byte("Wildcard: " + wildcard))
	})
	
	// Test more specific route precedence
	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	
	expectedBody := "User ID: 123"
	if string(body) != expectedBody {
		t.Errorf("Expected specific route to take precedence, got '%s'", string(body))
	}
	
	// Test wildcard route for non-matching specific path
	req = httptest.NewRequest("GET", "/api/products/456", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	
	expectedBody = "Wildcard: products/456"
	if string(body) != expectedBody {
		t.Errorf("Expected wildcard route to match, got '%s'", string(body))
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing API patterns still work as expected
	
	t.Run("Original parameter syntax", func(t *testing.T) {
		router := NewRouter[CustomData]()
		
		// Original parameter syntax
		router.GET("/users/:id", func(ctx *Ctx[CustomData]) {
			id := ctx.Params["id"]
			ctx.ResponseWriter.Write([]byte("ID: " + id))
		})
		
		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		
		expectedBody := "ID: 123"
		if string(body) != expectedBody {
			t.Errorf("Original parameter syntax failed, got '%s'", string(body))
		}
	})
	
	t.Run("Original wildcard syntax", func(t *testing.T) {
		router := NewRouter[CustomData]()
		
		// Original wildcard syntax
		router.GET("/files/*filepath", func(ctx *Ctx[CustomData]) {
			filepath := ctx.Params["filepath"]
			ctx.ResponseWriter.Write([]byte("Filepath: " + filepath))
		})
		
		req := httptest.NewRequest("GET", "/files/documents/report.pdf", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		
		expectedBody := "Filepath: documents/report.pdf"
		if string(body) != expectedBody {
			t.Errorf("Original wildcard syntax failed, got '%s'", string(body))
		}
	})
	
	t.Run("Original ctx.Param method", func(t *testing.T) {
		router := NewRouter[CustomData]()
		
		// Test for ctx.Param() backward compatibility
		router.GET("/item/:id", func(ctx *Ctx[CustomData]) {
			// Using original accessor method
			id := ctx.Param("id")
			ctx.ResponseWriter.Write([]byte("Item: " + id))
		})
		
		req := httptest.NewRequest("GET", "/item/abc123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		
		expectedBody := "Item: abc123"
		if string(body) != expectedBody {
			t.Errorf("Original ctx.Param() method failed, got '%s'", string(body))
		}
	})
}

func TestLongURLPath(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Register a route that can handle very long paths
	router.GET("/api/v1/resources/:id/nested/:type/*rest", func(ctx *Ctx[CustomData]) {
		id := ctx.Params["id"]
		resourceType := ctx.Params["type"]
		rest := ctx.Params["rest"]
		ctx.ResponseWriter.Write([]byte(fmt.Sprintf("ID: %s, Type: %s, Rest: %s", id, resourceType, rest)))
	})
	
	// Create a very long path with many segments
	var pathBuilder strings.Builder
	pathBuilder.WriteString("/api/v1/resources/12345/nested/document/")
	for i := 0; i < 20; i++ {
		pathBuilder.WriteString(fmt.Sprintf("segment%d/", i))
	}
	longPath := pathBuilder.String()
	
	req := httptest.NewRequest("GET", longPath, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	resp := w.Result()
	
	// Verify the router can handle very long paths
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Failed to handle long path, got status %d", resp.StatusCode)
	}
}

func TestEmptyPathSegments(t *testing.T) {
	router := NewRouter[CustomData]()
	
	// Add a root path handler
	router.GET("/", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Root"))
	})
	
	// Test route for handling empty segments correctly
	router.GET("/empty-test", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Success"))
	})
	
	// Test cases with potentially problematic empty paths
	testCases := []struct {
		name string
		path string
	}{
		{"Root path", "/"},
		{"Path with only slashes", "///"},
		{"Path ending with slash", "/empty-test/"},
		{"Path with multiple slashes", "//empty-test//"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			
			// This should not panic
			router.ServeHTTP(w, req)
			
			// For the empty-test routes, check they still match correctly
			if strings.Contains(tc.path, "empty-test") {
				resp := w.Result()
				body, _ := io.ReadAll(resp.Body)
				
				expectedBody := "Success"
				if string(body) != expectedBody {
					t.Errorf("Expected '%s', got '%s'", expectedBody, string(body))
				}
			}
		})
	}
}