package octo

import (
	"net/http"
	"runtime"
	"time"

	"github.com/Fy-/octypes"
)

// APIError represents an API error with a message and HTTP status code
type APIError struct {
	Message string
	Code    int
}

// BaseResult is the standard response format
type BaseResult struct {
	Data    interface{}         `json:"data,omitempty"`
	Time    float64             `json:"time"`
	Result  string              `json:"result"`
	Message string              `json:"message,omitempty"`
	Paging  *octypes.Pagination `json:"paging,omitempty"`
	Token   string              `json:"token,omitempty"`
}

// APIErrors is a map of error codes to APIError structs
var APIErrors = map[string]*APIError{
	"err_unknown_error":            {"Unknown error", http.StatusInternalServerError},
	"err_internal_error":           {"Internal error", http.StatusInternalServerError},
	"err_db_error":                 {"Database error", http.StatusInternalServerError},
	"err_invalid_request":          {"Invalid request", http.StatusBadRequest},
	"err_invalid_email_address":    {"Invalid email address", http.StatusBadRequest},
	"err_all_fields_are_mandatory": {"Missing required fields", http.StatusBadRequest},
	"err_email_not_configured":     {"Email not configured", http.StatusInternalServerError},
	"err_unauthorized":             {"Unauthorized", http.StatusUnauthorized},
	"err_not_found":                {"Not found", http.StatusNotFound},
	"err_invalid_uuid":             {"Invalid UUID", http.StatusBadRequest},
	"err_json_error":               {"JSON error", http.StatusBadRequest},
	// Add other error codes as needed
}

// SendError sends an error response based on the provided error code and error
func (c *Ctx[V]) SendError(code string, err error) {
	apiError, ok := APIErrors[code]
	if !ok {
		apiError = APIErrors["err_unknown_error"]
	}
	message := apiError.Message
	if err != nil {
		message += ": " + err.Error()
		if pc, file, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			logger.Error().Err(err).Msgf("[octo-error] error: %s in %s:%d %s\n", err.Error(), file, line, funcName)
		}
	}
	result := BaseResult{
		Result:  "error",
		Message: message,
		Token:   code,
		Time:    float64(time.Now().UnixNano()-c.StartTime) / 1e9,
	}
	c.SendJSON(apiError.Code, result)
}

func (c *Ctx[V]) SendErrorStatus(statusCode int, code string, err error) {
	apiError, ok := APIErrors[code]
	if !ok {
		apiError = APIErrors["err_unknown_error"]
	}
	message := apiError.Message
	if err != nil {
		message += ": " + err.Error()
		if pc, file, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			logger.Error().Err(err).Msgf("[octo-error] error: %s in %s:%d %s\n", err.Error(), file, line, funcName)
		}
	}
	result := BaseResult{
		Result:  "error",
		Message: message,
		Token:   code,
		Time:    float64(time.Now().UnixNano()-c.StartTime) / 1e9,
	}
	c.SendJSON(statusCode, result)
}

func (c *Ctx[V]) Redirect(status int, url string) {
	http.Redirect(c.ResponseWriter, c.Request, url, status)
}

// Send404 sends a 404 Not Found error response
func (c *Ctx[V]) Send404() {
	c.SendError("err_not_found", nil)
}

// Send401 sends a 401 Unauthorized error response
func (c *Ctx[V]) Send401() {
	c.SendError("err_unauthorized", nil)
}

// SendInvalidUUID sends an error response for invalid UUID
func (c *Ctx[V]) SendInvalidUUID() {
	c.SendError("err_invalid_uuid", nil)
}

// NewJSONResult sends a successful JSON response with optional pagination
func (c *Ctx[V]) NewJSONResult(data interface{}, pagination *octypes.Pagination) {
	result := BaseResult{
		Data:   data,
		Time:   float64(time.Now().UnixNano()-c.StartTime) / 1e9,
		Result: "success",
		Paging: pagination,
	}
	c.SendJSON(http.StatusOK, result)
}
func (c *Ctx[V]) SendData(statusCode int, contentType string, data []byte) {
	c.SetHeader("Content-Type", contentType)
	c.SetStatus(statusCode)
	c.ResponseWriter.Write(data)
}
