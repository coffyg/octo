package octo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// CustomData type for Ctx.Custom
type QueryTestCustomData struct {
	Field string
}

func TestComplexQueryValues(t *testing.T) {
	router := NewRouter[QueryTestCustomData]()

	// Add a route that handles a complex query parameter
	router.GET("/image", func(ctx *Ctx[QueryTestCustomData]) {
		// Get the complex query parameter
		vars := ctx.QueryValue("vars")
		ctx.SendString(http.StatusOK, vars)
	})

	// Test cases for different complex query values
	testCases := []struct {
		name         string
		query        string
		expectedBody string
	}{
		{
			name:         "Image transformation",
			query:        "?vars=format=webp:scale_crop_center=380x190",
			expectedBody: "format=webp:scale_crop_center=380x190",
		},
		{
			name:         "Multiple parameters",
			query:        "?vars=resize=200x300:quality=80:format=jpg",
			expectedBody: "resize=200x300:quality=80:format=jpg",
		},
		{
			name:         "URL encoded values",
			query:        "?vars=text=Hello%20World:position=center",
			expectedBody: "text=Hello World:position=center",
		},
		{
			name:         "Multiple query parameters",
			query:        "?vars=format=png&width=100&height=200",
			expectedBody: "format=png",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request with the complex query
			req := httptest.NewRequest("GET", "/image"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check the response
			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Get the response body
			body := w.Body.String()
			if body != tc.expectedBody {
				t.Errorf("Expected body '%s', got '%s'", tc.expectedBody, body)
			}
		})
	}
}

func TestQueryMap(t *testing.T) {
	router := NewRouter[QueryTestCustomData]()

	// Add a route that returns all query parameters as JSON
	router.GET("/query", func(ctx *Ctx[QueryTestCustomData]) {
		queryMap := ctx.QueryMap()
		ctx.SendJSON(http.StatusOK, queryMap)
	})

	// Test with a complex query string
	req := httptest.NewRequest("GET", "/query?vars=format=webp:scale_crop_center=380x190&width=100&height=200", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// The response should be a JSON object with the query parameters
	expectedContentType := "application/json"
	actualContentType := resp.Header.Get("Content-Type")
	if actualContentType != expectedContentType {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, actualContentType)
	}
}

func TestNestedQueryValues(t *testing.T) {
	router := NewRouter[QueryTestCustomData]()

	// Add a route that extracts values from a complex nested query parameter
	router.GET("/parse", func(ctx *Ctx[QueryTestCustomData]) {
		// Get the complex query parameter
		vars := ctx.QueryValue("vars")

		// For testing - just return the raw query value
		// In a real implementation, you would parse this into key-value pairs
		ctx.SendString(http.StatusOK, vars)
	})

	// Test with a complex nested query string
	req := httptest.NewRequest("GET", "/parse?vars=format=webp:scale_crop_center=380x190:quality=80", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// The response should be exactly the value of the vars parameter
	expectedBody := "format=webp:scale_crop_center=380x190:quality=80"
	actualBody := w.Body.String()
	if actualBody != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, actualBody)
	}
}
