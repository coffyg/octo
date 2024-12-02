package octo

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/coffyg/octypes"

	"github.com/go-playground/form/v4"
)

var formDecoder = form.NewDecoder()

type Ctx[V any] struct {
	ResponseWriter *ResponseWriterWrapper `json:"-"`
	Request        *http.Request          `json:"-"`
	Params         map[string]string      `json:"Params"`
	Query          map[string][]string    `json:"Query"`
	StartTime      int64                  `json:"StartTime"`
	UUID           string                 `json:"UUID"`
	Body           []byte                 `json:"-"`
	Headers        http.Header            `json:"-"`
	Custom         V                      // Generic Custom Field
	done           bool
	hasReadBody    bool
}

func (c *Ctx[V]) SetHeader(key, value string) {
	c.ResponseWriter.Header().Set(key, value)
}

func (c *Ctx[V]) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Ctx[V]) DelHeader(key string) {
	c.ResponseWriter.Header().Del(key)
}

func (c *Ctx[V]) GetParam(key string) string {
	return c.Params[key]
}

func (c *Ctx[V]) SetParam(key, value string) {
	c.Params[key] = value
}

func (c *Ctx[V]) SetStatus(code int) {
	c.ResponseWriter.WriteHeader(code)
}

func (c *Ctx[V]) JSON(statusCode int, v interface{}) {
	c.SendJSON(statusCode, v)
}

func (c *Ctx[V]) SendJSON(statusCode int, v interface{}) {
	if c.done {
		return
	}
	response, err := json.Marshal(v)
	if err != nil {
		c.SendError("err_json_error", err)
		return
	}
	c.SetHeader("Content-Type", "application/json")
	c.SetHeader("Content-Length", strconv.Itoa(len(response)))
	c.SetStatus(statusCode)
	_, err = c.ResponseWriter.Write(response)
	if err != nil {
		logger.Error().Err(err).Msg("[octo] failed to write response")
	}
	c.Done()
}

func (c *Ctx[V]) Param(key string) string {
	if value, ok := c.Params[key]; ok {
		return value
	}

	return ""
}

func (c *Ctx[V]) QueryParam(key string) string {
	if value, ok := c.Params[key]; ok {
		return value
	}

	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c *Ctx[V]) DefaultQueryParam(key, defaultValue string) string {
	if value, ok := c.Params[key]; ok {
		return value
	}

	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}

	return defaultValue
}

func (c *Ctx[V]) QueryValue(key string) string {
	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c *Ctx[V]) DefaultQuery(key, defaultValue string) string {
	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}

	return defaultValue
}

func (c *Ctx[V]) QueryArray(key string) []string {
	return c.Request.URL.Query()[key]
}

func (c *Ctx[V]) QueryMap() map[string][]string {
	return c.Request.URL.Query()
}

func (c *Ctx[V]) Context() context.Context {
	return c.Request.Context()
}

func (c *Ctx[V]) Done() {
	if c.done {
		return
	}
	/*
		c.ResponseWriter.Flush()
		if c.Request.Body != nil {
			c.Request.Body.Close()
			c.Request.Body = nil
		}
	*/
	c.done = true
}

func (c *Ctx[V]) IsDone() bool {
	return c.done
}

// ClientIP returns the client's IP address, even if behind a proxy
func (c *Ctx[V]) ClientIP() string {
	// X-Forwarded-For may contain multiple IPs
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		ips := strings.Split(ip, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			parsedIP := net.ParseIP(ip)
			if parsedIP != nil {
				return ip
			}
		}
	}

	// Fallback to X-Real-IP
	ip = c.GetHeader("X-Real-IP")
	if ip != "" {
		ip = strings.TrimSpace(ip)
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil {
			return ip
		}
	}

	// Finally, use RemoteAddr
	ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// Cookie retrieves the value of the named cookie from the request.
// It returns an error if the cookie is not present.
func (c *Ctx[V]) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie adds a Set-Cookie header to the response.
func (c *Ctx[V]) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if maxAge <= 0 {
		maxAge = -1
	}
	if path == "" {
		path = "/"
	}
	if domain == "" {
		domain = c.Request.URL.Hostname()
	}
	http.SetCookie(c.ResponseWriter, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// BindJSON binds the request body into an interface.
func (c *Ctx[V]) ShouldBindJSON(obj interface{}) error {
	err := c.NeedBody()
	if err != nil {
		return err
	}
	if len(c.Body) == 0 {
		return errors.New("request body is empty")
	}
	return json.Unmarshal(c.Body, obj)
}

// ShouldBindXML binds the XML request body into the provided object.
func (c *Ctx[V]) ShouldBindXML(obj interface{}) error {
	err := c.NeedBody()
	if err != nil {
		return err
	}
	if len(c.Body) == 0 {
		return errors.New("request body is empty")
	}
	return xml.Unmarshal(c.Body, obj)
}

// ShouldBindForm binds form data into the provided object.
func (c *Ctx[V]) ShouldBindForm(obj interface{}) error {
	err := c.NeedBody()
	if err != nil {
		return err
	}
	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	values := c.Request.PostForm
	return mapForm(obj, values)
}

// ShouldBindMultipartForm binds multipart form data into the provided object.
func (c *Ctx[V]) ShouldBindMultipartForm(obj interface{}) error {
	err := c.NeedBody()
	if err != nil {
		return err
	}
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return err
	}
	values := c.Request.MultipartForm.Value
	return mapForm(obj, values)
}

// mapForm maps form values into the provided struct.
func mapForm(ptr interface{}, formData url.Values) error {
	return formDecoder.Decode(ptr, formData)
}

// ShouldBind binds the request body into the provided object
// according to the Content-Type header.
func (c *Ctx[V]) ShouldBind(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	contentType, _, _ = mime.ParseMediaType(contentType)

	switch contentType {
	case "application/json":
		return c.ShouldBindJSON(obj)
	case "application/xml", "text/xml":
		return c.ShouldBindXML(obj)
	case "application/x-www-form-urlencoded":
		return c.ShouldBindForm(obj)
	case "multipart/form-data":
		return c.ShouldBindMultipartForm(obj)
	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func (c *Ctx[V]) NeedBody() error {
	if c.hasReadBody {
		return nil
	}
	c.hasReadBody = true
	c.ResponseWriter.CaptureBody = true

	limitedReader := io.LimitReader(c.Request.Body, maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		logger.Error().Err(err).Msg("[octo] failed to read request body")
		return err
	}

	if int64(len(body)) > maxBodySize {
		err := errors.New("request body too large")
		logger.Error().Err(err).Msg("[octo] request body exceeds maximum allowed size")
		return err
	}

	c.Request.Body.Close()
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Body = body
	return nil
}

// SendError sends an error response based on the provided error code and error
func (c *Ctx[V]) SendError(code string, err error) {
	if c.done {
		return
	}
	apiError, ok := APIErrors[code]
	if !ok {
		apiError = APIErrors["err_unknown_error"]
	}
	message := apiError.Message
	if err != nil {
		message += ": " + err.Error()
		if pc, file, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			logger.Error().Err(err).Msgf("[octo-error] error: %s in %s:%d %s", err.Error(), file, line, funcName)
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

// SendErrorStatus sends an error response with a specific HTTP status code
func (c *Ctx[V]) SendErrorStatus(statusCode int, code string, err error) {
	if c.done {
		return
	}
	apiError, ok := APIErrors[code]
	if !ok {
		apiError = APIErrors["err_unknown_error"]
	}
	message := apiError.Message
	if err != nil {
		message += ": " + err.Error()
		if pc, file, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			logger.Error().Err(err).Msgf("[octo-error] error: %s in %s:%d %s", err.Error(), file, line, funcName)
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
	if c.done {
		return
	}
	http.Redirect(c.ResponseWriter, c.Request, url, status)
	c.Done()
}

// Send404 sends a 404 Not Found error response
func (c *Ctx[V]) Send404() {
	if c.done {
		return
	}
	c.SendError("err_not_found", nil)
}

// Send401 sends a 401 Unauthorized error response
func (c *Ctx[V]) Send401() {
	if c.done {
		return
	}
	c.SendError("err_unauthorized", nil)
}

// SendInvalidUUID sends an error response for invalid UUID
func (c *Ctx[V]) SendInvalidUUID() {
	if c.done {
		return
	}
	c.SendError("err_invalid_uuid", nil)
}

// NewJSONResult sends a successful JSON response with optional pagination
func (c *Ctx[V]) NewJSONResult(data interface{}, pagination *octypes.Pagination) {
	if c.done {
		return
	}
	result := BaseResult{
		Data:   data,
		Time:   float64(time.Now().UnixNano()-c.StartTime) / 1e9,
		Result: "success",
		Paging: pagination,
	}
	c.SendJSON(http.StatusOK, result)
}

// SendData sends a response with the provided status code and data
func (c *Ctx[V]) SendData(statusCode int, contentType string, data []byte) {
	if c.done {
		return
	}
	c.SetHeader("Content-Type", contentType)
	c.SetHeader("Content-Length", strconv.Itoa(len(data)))
	c.SetStatus(statusCode)
	_, err := c.ResponseWriter.Write(data)
	if err != nil {
		logger.Error().Err(err).Msg("[octo] failed to write data")
	}
	c.Done()
}

// Send a file as response
func (c *Ctx[V]) File(urlPath string, filePath string) {
	if c.done {
		return
	}
	if filePath == "" {
		c.SendError("err_internal_error", fmt.Errorf("file path is empty"))
		return
	}

	http.ServeFile(c.ResponseWriter, c.Request, filePath)
	c.Done()
}

// Send a file as response from a http.FileSystem
func (c *Ctx[V]) FileFromFS(urlPath string, fs http.FileSystem, filePath string) {
	if c.done {
		return
	}
	if fs == nil {
		c.SendError("err_internal_error", fmt.Errorf("http.FileSystem is nil"))
		return
	}
	http.FileServer(fs).ServeHTTP(c.ResponseWriter, c.Request)
	c.Done()
}

// FormValue retrieves form values from the request
func (c *Ctx[V]) FormValue(key string) string {
	if c.Request.Form == nil {
		c.Request.ParseForm()
	}
	return c.Request.FormValue(key)
}

// SendString sends a string response
func (c *Ctx[V]) SendString(statusCode int, s string) {
	c.SendData(statusCode, "text/plain", []byte(s))
}

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
