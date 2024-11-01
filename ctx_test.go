package octo

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestGlobalMiddlewareOrder(t *testing.T) {
	router := NewRouter[CustomData]()

	// Variables to track the order of middleware execution
	executionOrder := []string{}

	// Global middleware
	router.UseGlobal(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			executionOrder = append(executionOrder, "global")
			next(ctx)
		}
	})

	// Middleware to test group application
	groupMiddleware := func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			executionOrder = append(executionOrder, "group")
			next(ctx)
		}
	}

	// Create a group with middleware
	apiGroup := router.Group("/api", groupMiddleware)

	// Add a route to the group
	apiGroup.GET("/test", func(ctx *Ctx[CustomData]) {
		executionOrder = append(executionOrder, "handler")
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Send request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check the execution order
	expectedOrder := []string{"global", "group", "handler"}
	if !reflect.DeepEqual(executionOrder, expectedOrder) {
		t.Errorf("Expected execution order %v, got %v", expectedOrder, executionOrder)
	}
}

func TestCookieMethods(t *testing.T) {
	router := NewRouter[CustomData]()

	router.GET("/cookie", func(ctx *Ctx[CustomData]) {
		// Try to get the cookie
		value, err := ctx.Cookie("test_cookie")
		if err != nil {
			// Set the cookie if not present
			ctx.SetCookie("test_cookie", "test_value", 3600, "/", "", false, true)
			ctx.ResponseWriter.Write([]byte("Cookie Set"))
		} else {
			ctx.ResponseWriter.Write([]byte("Cookie Value: " + value))
		}
	})

	// First request to set the cookie
	req := httptest.NewRequest("GET", "/cookie", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Cookie Set" {
		t.Errorf("Expected 'Cookie Set', got '%s'", string(body))
	}

	// Extract the Set-Cookie header
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("Expected a Set-Cookie header")
	}

	// Second request with the cookie
	req = httptest.NewRequest("GET", "/cookie", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	if string(body) != "Cookie Value: test_value" {
		t.Errorf("Expected 'Cookie Value: test_value', got '%s'", string(body))
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

func TestShouldBind(t *testing.T) {
	router := NewRouter[CustomData]()

	type TestData struct {
		Name string `json:"name" form:"name" xml:"name"`
		Age  int    `json:"age" form:"age" xml:"age"`
	}

	router.POST("/bind", func(ctx *Ctx[CustomData]) {
		var data TestData
		if err := ctx.ShouldBind(&data); err != nil {
			ctx.SendJSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}
		ctx.SendJSON(http.StatusOK, data)
	})

	// Test with JSON
	jsonData := `{"name": "John", "age": 30}`
	req := httptest.NewRequest("POST", "/bind", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	var responseData TestData
	if err := json.Unmarshal(body, &responseData); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if responseData.Name != "John" || responseData.Age != 30 {
		t.Errorf("Unexpected response data: %+v", responseData)
	}

	// Test with form data
	form := url.Values{}
	form.Add("name", "Jane")
	form.Add("age", "25")
	req = httptest.NewRequest("POST", "/bind", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &responseData); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if responseData.Name != "Jane" || responseData.Age != 25 {
		t.Errorf("Unexpected response data: %+v", responseData)
	}

	// Test with multipart form data
	var b bytes.Buffer
	wr := multipart.NewWriter(&b)
	wr.WriteField("name", "Alice")
	wr.WriteField("age", "28")
	wr.Close()
	req = httptest.NewRequest("POST", "/bind", &b)
	req.Header.Set("Content-Type", wr.FormDataContentType())
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &responseData); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if responseData.Name != "Alice" || responseData.Age != 28 {
		t.Errorf("Unexpected response data: %+v", responseData)
	}

	// Test with XML
	xmlData := `<TestData><name>Bob</name><age>35</age></TestData>`
	req = httptest.NewRequest("POST", "/bind", strings.NewReader(xmlData))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	resp = w.Result()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &responseData); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if responseData.Name != "Bob" || responseData.Age != 35 {
		t.Errorf("Unexpected response data: %+v", responseData)
	}
}
