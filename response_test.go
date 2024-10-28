package octo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// CustomData is a placeholder for your custom context data

func TestSendError(t *testing.T) {
	router := NewRouter[CustomData]()

	router.GET("/test_error", func(ctx *Ctx[CustomData]) {
		ctx.SendError("err_invalid_request", nil)
	})

	req := httptest.NewRequest("GET", "/test_error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var result BaseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Result != "error" || result.Token != "err_invalid_request" {
		t.Errorf("Unexpected response: %+v", result)
	}
}

func TestSend404(t *testing.T) {
	router := NewRouter[CustomData]()

	router.GET("/test_404", func(ctx *Ctx[CustomData]) {
		ctx.Send404()
	})

	req := httptest.NewRequest("GET", "/test_404", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}

	// Similar checks as in TestSendError
}

func TestNewJSONResult(t *testing.T) {
	router := NewRouter[CustomData]()

	router.GET("/test_success", func(ctx *Ctx[CustomData]) {
		data := map[string]string{"key": "value"}
		ctx.NewJSONResult(data, nil)
	})

	req := httptest.NewRequest("GET", "/test_success", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result BaseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Result != "success" || result.Data == nil {
		t.Errorf("Unexpected response: %+v", result)
	}
}
