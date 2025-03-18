package octo

import (
    "bytes"
    "context"
    "encoding/json"
    "encoding/xml"
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
    "github.com/pkg/errors"
)

// formDecoder is a global instance of the form decoder
var formDecoder = form.NewDecoder()

// Ctx represents the HTTP request context with generic custom data
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

// SetHeader sets a response header with the given key and value
func (ctx *Ctx[V]) SetHeader(key, value string) {
    ctx.ResponseWriter.Header().Set(key, value)
}

// GetHeader returns a request header value for the given key
func (ctx *Ctx[V]) GetHeader(key string) string {
    return ctx.Request.Header.Get(key)
}

// DelHeader removes a response header with the given key
func (ctx *Ctx[V]) DelHeader(key string) {
    ctx.ResponseWriter.Header().Del(key)
}

// GetParam returns a path parameter value for the given key
func (ctx *Ctx[V]) GetParam(key string) string {
    return ctx.Params[key]
}

// SetParam sets a path parameter with the given key and value
func (ctx *Ctx[V]) SetParam(key, value string) {
    ctx.Params[key] = value
}

// SetStatus sets the HTTP response status code
func (ctx *Ctx[V]) SetStatus(code int) {
    ctx.ResponseWriter.WriteHeader(code)
}

// JSON sends a JSON response with the given status code and value
func (ctx *Ctx[V]) JSON(statusCode int, v interface{}) {
    ctx.SendJSON(statusCode, v)
}

// SendJSON marshals the given value to JSON and sends it as response
func (ctx *Ctx[V]) SendJSON(statusCode int, v interface{}) {
    if ctx.done {
        return
    }
    
    response, err := json.Marshal(v)
    if err != nil {
        ctx.SendError("err_json_error", err)
        return
    }
    
    ctx.SetHeader("Content-Type", "application/json")
    ctx.SetHeader("Content-Length", strconv.Itoa(len(response)))
    ctx.SetStatus(statusCode)
    
    _, err = ctx.ResponseWriter.Write(response)
    if err != nil {
        if EnableLoggerCheck {
            if logger != nil {
                logger.Error().Err(err).Msg("[octo] failed to write response")
            }
        } else {
            logger.Error().Err(err).Msg("[octo] failed to write response")
        }
    }
    
    ctx.Done()
}

// Param returns a path parameter value for the given key
func (ctx *Ctx[V]) Param(key string) string {
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    return ""
}

// QueryParam returns a query parameter value, checking both
// path parameters and URL query parameters
func (ctx *Ctx[V]) QueryParam(key string) string {
    // First check path params
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    
    // Then check query params
    values := ctx.Request.URL.Query()[key]
    if len(values) > 0 {
        return values[0]
    }
    
    return ""
}

// DefaultQueryParam returns a query parameter value with a default fallback
func (ctx *Ctx[V]) DefaultQueryParam(key, defaultValue string) string {
    // First check path params
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    
    // Then check query params
    values := ctx.Request.URL.Query()[key]
    if len(values) > 0 {
        return values[0]
    }
    
    return defaultValue
}

// QueryValue returns only a URL query parameter value
// (does not check path parameters)
func (ctx *Ctx[V]) QueryValue(key string) string {
    values := ctx.Request.URL.Query()[key]
    if len(values) > 0 {
        return values[0]
    }
    return ""
}

// DefaultQuery returns a URL query parameter with a default fallback
func (ctx *Ctx[V]) DefaultQuery(key, defaultValue string) string {
    values := ctx.Request.URL.Query()[key]
    if len(values) > 0 {
        return values[0]
    }
    return defaultValue
}

// QueryArray returns all values for a URL query parameter
func (ctx *Ctx[V]) QueryArray(key string) []string {
    return ctx.Request.URL.Query()[key]
}

// QueryMap returns the entire URL query parameters map
func (ctx *Ctx[V]) QueryMap() map[string][]string {
    return ctx.Request.URL.Query()
}

// Context returns the native Go context from the request
func (ctx *Ctx[V]) Context() context.Context {
    return ctx.Request.Context()
}

// Done marks the context as completed to prevent further processing
func (ctx *Ctx[V]) Done() {
    if ctx.done {
        return
    }
    ctx.done = true
}

// IsDone returns whether the context has been marked as done
func (ctx *Ctx[V]) IsDone() bool {
    return ctx.done
}

// ClientIP returns the client's IP address, even if behind a proxy
func (ctx *Ctx[V]) ClientIP() string {
    // Try to get IP from X-Forwarded-For header first
    if ip := ctx.clientIPFromXForwardedFor(); ip != "" {
        return ip
    }
    
    // Try to get IP from X-Real-IP header next
    if ip := ctx.clientIPFromXRealIP(); ip != "" {
        return ip
    }
    
    // Finally, use the direct RemoteAddr
    return ctx.clientIPFromRemoteAddr()
}

// clientIPFromXForwardedFor attempts to extract a valid IP from the X-Forwarded-For header
func (ctx *Ctx[V]) clientIPFromXForwardedFor() string {
    ip := ctx.GetHeader("X-Forwarded-For")
    if ip == "" {
        return ""
    }
    
    ips := strings.Split(ip, ",")
    for _, ipStr := range ips {
        ipStr = strings.TrimSpace(ipStr)
        parsedIP := net.ParseIP(ipStr)
        if parsedIP != nil {
            return ipStr
        }
    }
    
    return ""
}

// clientIPFromXRealIP attempts to extract a valid IP from the X-Real-IP header
func (ctx *Ctx[V]) clientIPFromXRealIP() string {
    ip := ctx.GetHeader("X-Real-IP")
    if ip == "" {
        return ""
    }
    
    ip = strings.TrimSpace(ip)
    parsedIP := net.ParseIP(ip)
    if parsedIP != nil {
        return ip
    }
    
    return ""
}

// clientIPFromRemoteAddr extracts the IP part from the RemoteAddr
func (ctx *Ctx[V]) clientIPFromRemoteAddr() string {
    ip, _, err := net.SplitHostPort(strings.TrimSpace(ctx.Request.RemoteAddr))
    if err != nil {
        return ctx.Request.RemoteAddr
    }
    return ip
}

// Cookie retrieves the value of the named cookie from the request
func (ctx *Ctx[V]) Cookie(name string) (string, error) {
    cookie, err := ctx.Request.Cookie(name)
    if err != nil {
        return "", errors.Wrap(err, "cookie not found")
    }
    return cookie.Value, nil
}

// SetCookie adds a Set-Cookie header to the response
func (ctx *Ctx[V]) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
    if maxAge <= 0 {
        maxAge = -1
    }
    
    if path == "" {
        path = "/"
    }
    
    if domain == "" {
        domain = ctx.Request.URL.Hostname()
    }
    
    http.SetCookie(ctx.ResponseWriter, &http.Cookie{
        Name:     name,
        Value:    value,
        MaxAge:   maxAge,
        Path:     path,
        Domain:   domain,
        Secure:   secure,
        HttpOnly: httpOnly,
    })
}

// ShouldBindJSON binds the request body into an interface using JSON
func (ctx *Ctx[V]) ShouldBindJSON(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return errors.Wrap(err, "failed to read request body")
    }
    
    if len(ctx.Body) == 0 {
        return errors.New("request body is empty")
    }
    
    return errors.Wrap(
        json.Unmarshal(ctx.Body, obj),
        "failed to unmarshal JSON",
    )
}

// ShouldBindXML binds the request body into an interface using XML
func (ctx *Ctx[V]) ShouldBindXML(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return errors.Wrap(err, "failed to read request body")
    }
    
    if len(ctx.Body) == 0 {
        return errors.New("request body is empty")
    }
    
    return errors.Wrap(
        xml.Unmarshal(ctx.Body, obj),
        "failed to unmarshal XML",
    )
}

// ShouldBindForm binds form data into the provided object
func (ctx *Ctx[V]) ShouldBindForm(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return errors.Wrap(err, "failed to read request body")
    }
    
    if err := ctx.Request.ParseForm(); err != nil {
        return errors.Wrap(err, "failed to parse form")
    }
    
    values := ctx.Request.PostForm
    return errors.Wrap(
        mapForm(obj, values),
        "failed to map form values",
    )
}

// ShouldBindMultipartForm binds multipart form data into the provided object
func (ctx *Ctx[V]) ShouldBindMultipartForm(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return errors.Wrap(err, "failed to read request body")
    }
    
    // 32MB max memory
    if err := ctx.Request.ParseMultipartForm(32 << 20); err != nil {
        return errors.Wrap(err, "failed to parse multipart form")
    }
    
    values := ctx.Request.MultipartForm.Value
    return errors.Wrap(
        mapForm(obj, values),
        "failed to map multipart form values",
    )
}

// mapForm maps form values into the provided struct
func mapForm(ptr interface{}, formData url.Values) error {
    return formDecoder.Decode(ptr, formData)
}

// ShouldBind binds the request body into the provided object
// according to the Content-Type header
func (ctx *Ctx[V]) ShouldBind(obj interface{}) error {
    contentType := ctx.GetHeader("Content-Type")
    contentType, _, _ = mime.ParseMediaType(contentType)

    switch contentType {
    case "application/json":
        return ctx.ShouldBindJSON(obj)
    case "application/xml", "text/xml":
        return ctx.ShouldBindXML(obj)
    case "application/x-www-form-urlencoded":
        return ctx.ShouldBindForm(obj)
    case "multipart/form-data":
        return ctx.ShouldBindMultipartForm(obj)
    default:
        return fmt.Errorf("unsupported content type: %s", contentType)
    }
}

// NeedBody ensures the request body is loaded and available for processing
func (ctx *Ctx[V]) NeedBody() error {
    if ctx.hasReadBody {
        return nil
    }
    
    ctx.hasReadBody = true
    ctx.ResponseWriter.CaptureBody = true

    // Read body with size limit
    err := ctx.readBodyWithSizeLimit()
    if err != nil {
        return err
    }
    
    return nil
}

// readBodyWithSizeLimit reads the request body with size limit check
func (ctx *Ctx[V]) readBodyWithSizeLimit() error {
    // Add 1 byte to detect if the body is too large
    limitedReader := io.LimitReader(ctx.Request.Body, GetMaxBodySize()+1)
    body, err := io.ReadAll(limitedReader)
    
    if err != nil {
        ctx.logBodyReadError(err)
        return errors.Wrap(err, "failed to read request body")
    }

    // Check if body exceeds maximum size
    if int64(len(body)) > GetMaxBodySize() {
        err := errors.New("request body too large")
        ctx.logBodySizeError(err)
        return err
    }

    // Save the body and create a new reader for the request
    ctx.Request.Body.Close()
    ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
    ctx.Body = body
    
    return nil
}

// logBodyReadError logs body read errors with logger check
func (ctx *Ctx[V]) logBodyReadError(err error) {
    if EnableLoggerCheck {
        if logger != nil {
            logger.Error().Err(err).Msg("[octo] failed to read request body")
        }
    } else {
        logger.Error().Err(err).Msg("[octo] failed to read request body")
    }
}

// logBodySizeError logs body size errors with logger check
func (ctx *Ctx[V]) logBodySizeError(err error) {
    if EnableLoggerCheck {
        if logger != nil {
            logger.Error().
                Err(err).
                Int64("maxSize", GetMaxBodySize()).
                Msg("[octo] request body exceeds maximum allowed size")
        }
    } else {
        logger.Error().
            Err(err).
            Int64("maxSize", GetMaxBodySize()).
            Msg("[octo] request body exceeds maximum allowed size")
    }
}

// SendError sends an error response based on the provided error code and error
func (ctx *Ctx[V]) SendError(code string, err error) {
    if ctx.done {
        return
    }
    
    // Get the API error definition or fall back to unknown error
    apiError, ok := APIErrors[code]
    if !ok {
        apiError = APIErrors["err_unknown_error"]
    }
    
    // Build the error message and response
    message := ctx.buildErrorMessage(apiError.Message, err)
    result := ctx.createErrorResult(code, message)
    
    // Send the JSON response with the appropriate status code
    ctx.SendJSON(apiError.Code, result)
}

// SendErrorStatus sends an error response with a specific HTTP status code
func (ctx *Ctx[V]) SendErrorStatus(statusCode int, code string, err error) {
    if ctx.done {
        return
    }
    
    // Get the API error definition or fall back to unknown error
    apiError, ok := APIErrors[code]
    if !ok {
        apiError = APIErrors["err_unknown_error"]
    }
    
    // Build the error message and response
    message := ctx.buildErrorMessage(apiError.Message, err)
    result := ctx.createErrorResult(code, message)
    
    // Send the JSON response with the provided status code
    ctx.SendJSON(statusCode, result)
}

// buildErrorMessage creates the error message and logs the error if provided
func (ctx *Ctx[V]) buildErrorMessage(baseMessage string, err error) string {
    if err == nil {
        return baseMessage
    }
    
    // Append the error message to the base message
    message := baseMessage + ": " + err.Error()
    
    // Log the error with caller information
    if pc, file, line, ok := runtime.Caller(2); ok {
        funcName := runtime.FuncForPC(pc).Name()
        ctx.logError(err, file, line, funcName)
    }
    
    return message
}

// logError logs an error with caller information
func (ctx *Ctx[V]) logError(err error, file string, line int, funcName string) {
    if EnableLoggerCheck {
        if logger != nil {
            logger.Error().
                Err(err).
                Str("file", file).
                Int("line", line).
                Str("function", funcName).
                Msg("[octo-error] API error occurred")
        }
    } else {
        logger.Error().
            Err(err).
            Str("file", file).
            Int("line", line).
            Str("function", funcName).
            Msg("[octo-error] API error occurred")
    }
}

// createErrorResult creates a BaseResult for error responses
func (ctx *Ctx[V]) createErrorResult(code string, message string) BaseResult {
    return BaseResult{
        Result:  "error",
        Message: message,
        Token:   code,
        Time:    float64(time.Now().UnixNano()-ctx.StartTime) / 1e9,
    }
}

// Redirect performs an HTTP redirect to the specified URL
func (ctx *Ctx[V]) Redirect(status int, url string) {
    if ctx.done {
        return
    }
    http.Redirect(ctx.ResponseWriter, ctx.Request, url, status)
    ctx.Done()
}

// Send404 sends a 404 Not Found error response
func (ctx *Ctx[V]) Send404() {
    if ctx.done {
        return
    }
    ctx.SendError("err_not_found", nil)
}

// Send401 sends a 401 Unauthorized error response
func (ctx *Ctx[V]) Send401() {
    if ctx.done {
        return
    }
    ctx.SendError("err_unauthorized", nil)
}

// SendInvalidUUID sends an error response for invalid UUID
func (ctx *Ctx[V]) SendInvalidUUID() {
    if ctx.done {
        return
    }
    ctx.SendError("err_invalid_uuid", nil)
}

// NewJSONResult sends a successful JSON response with optional pagination
func (ctx *Ctx[V]) NewJSONResult(data interface{}, pagination *octypes.Pagination) {
    if ctx.done {
        return
    }
    
    result := BaseResult{
        Data:   data,
        Time:   float64(time.Now().UnixNano()-ctx.StartTime) / 1e9,
        Result: "success",
        Paging: pagination,
    }
    
    ctx.SendJSON(http.StatusOK, result)
}

// SendData sends a response with the provided status code, content type, and data
func (ctx *Ctx[V]) SendData(statusCode int, contentType string, data []byte) {
    if ctx.done {
        return
    }
    
    // Set headers
    ctx.SetHeader("Content-Type", contentType)
    ctx.SetHeader("Content-Length", strconv.Itoa(len(data)))
    ctx.SetStatus(statusCode)
    
    // Write data
    _, err := ctx.ResponseWriter.Write(data)
    if err != nil {
        ctx.logDataWriteError(err)
    }
    
    ctx.Done()
}

// logDataWriteError logs errors that occur when writing response data
func (ctx *Ctx[V]) logDataWriteError(err error) {
    path := ctx.Request.URL.Path
    clientIP := ctx.ClientIP()
    
    if EnableLoggerCheck {
        if logger != nil {
            logger.Error().
                Err(err).
                Str("ip", clientIP).
                Str("path", path).
                Msg("[octo] failed to write response data")
        }
    } else {
        logger.Error().
            Err(err).
            Str("ip", clientIP).
            Str("path", path).
            Msg("[octo] failed to write response data")
    }
}

// File serves a file as the HTTP response
func (ctx *Ctx[V]) File(urlPath string, filePath string) {
    if ctx.done {
        return
    }
    
    if filePath == "" {
        err := fmt.Errorf("file path is empty")
        ctx.SendError("err_internal_error", err)
        return
    }
    
    http.ServeFile(ctx.ResponseWriter, ctx.Request, filePath)
    ctx.Done()
}

// FileFromFS serves a file from a http.FileSystem as the HTTP response
func (ctx *Ctx[V]) FileFromFS(urlPath string, fs http.FileSystem, filePath string) {
    if ctx.done {
        return
    }
    
    if fs == nil {
        err := fmt.Errorf("http.FileSystem is nil")
        ctx.SendError("err_internal_error", err)
        return
    }
    
    http.FileServer(fs).ServeHTTP(ctx.ResponseWriter, ctx.Request)
    ctx.Done()
}

// FormValue retrieves form values from the request
func (ctx *Ctx[V]) FormValue(key string) string {
    if ctx.Request.Form == nil {
        err := ctx.Request.ParseForm()
        if err != nil && EnableLoggerCheck && logger != nil {
            logger.Warn().
                Err(err).
                Str("key", key).
                Msg("[octo] failed to parse form")
        }
    }
    
    return ctx.Request.FormValue(key)
}

// SendString sends a plain text string response
func (ctx *Ctx[V]) SendString(statusCode int, s string) {
    ctx.SendData(statusCode, "text/plain", []byte(s))
}

// APIError represents an API error with a message and HTTP status code
type APIError struct {
    Message string
    Code    int
}

// BaseResult is the standard response format for all API responses
type BaseResult struct {
    Data    interface{}         `json:"data,omitempty"`
    Time    float64             `json:"time"`
    Result  string              `json:"result"`
    Message string              `json:"message,omitempty"`
    Paging  *octypes.Pagination `json:"paging,omitempty"`
    Token   string              `json:"token,omitempty"`
}

// APIErrors defines standard error types with messages and status codes
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
