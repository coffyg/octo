package octo

import (
    // Standard library imports
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
    "strconv"
    "strings"
    "time"

    // Third-party imports
    "github.com/go-playground/form/v4"
    "github.com/pkg/errors"
    
    // Internal imports
    "github.com/coffyg/octypes"
)

// Global form decoder instance
var formDecoder = form.NewDecoder()

// HTTP request context with typed custom data field
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

func (ctx *Ctx[V]) SetHeader(key, value string) {
    ctx.ResponseWriter.Header().Set(key, value)
}

func (ctx *Ctx[V]) GetHeader(key string) string {
    return ctx.Request.Header.Get(key)
}

func (ctx *Ctx[V]) DelHeader(key string) {
    ctx.ResponseWriter.Header().Del(key)
}

func (ctx *Ctx[V]) GetParam(key string) string {
    return ctx.Params[key]
}

func (ctx *Ctx[V]) SetParam(key, value string) {
    ctx.Params[key] = value
}

func (ctx *Ctx[V]) SetStatus(code int) {
    ctx.ResponseWriter.WriteHeader(code)
}

func (ctx *Ctx[V]) JSON(statusCode int, v interface{}) error {
    return ctx.SendJSON(statusCode, v)
}

func (ctx *Ctx[V]) SendJSON(statusCode int, v interface{}) error {
    if ctx.done {
        return nil
    }

    // Validate input
    if statusCode < 100 || statusCode > 599 {
        ctx.SendError("err_invalid_status", New(ErrInvalidRequest, fmt.Sprintf("Invalid HTTP status code: %d", statusCode)))
        return fmt.Errorf("invalid HTTP status code: %d", statusCode)
    }
    
    response, err := json.Marshal(v)
    if err != nil {
        ctx.SendError("err_json_error", err)
        return err
    }
    
    ctx.SetHeader("Content-Type", "application/json")
    ctx.SetHeader("Content-Length", strconv.Itoa(len(response)))
    ctx.SetStatus(statusCode)
    
    _, err = ctx.ResponseWriter.Write(response)
    if err != nil {
        // Simplify nested conditionals
        if EnableLoggerCheck && logger == nil {
            // Skip logging if logger is disabled
        } else {
            logger.Error().
                Err(err).
                Str("path", ctx.Request.URL.Path).
                Str("ip", ctx.ClientIP()).
                Msg("[octo] failed to write response")
        }
        ctx.Done()
        return err
    }
    
    ctx.Done()
    return nil
}

func (ctx *Ctx[V]) Param(key string) string {
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    return ""
}

// Checks both path and URL query parameters
func (ctx *Ctx[V]) QueryParam(key string) string {
    // First check path params (faster)
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    // Then check query params
    values := ctx.Query[key]
    if len(values) > 0 {
        return values[0]
    }
    
    return ""
}

func (ctx *Ctx[V]) DefaultQueryParam(key, defaultValue string) string {
    // First check path params (faster)
    if value, ok := ctx.Params[key]; ok {
        return value
    }
    
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    // Then check query params
    values := ctx.Query[key]
    if len(values) > 0 {
        return values[0]
    }
    
    return defaultValue
}

// Only checks URL query parameters, not path parameters
func (ctx *Ctx[V]) QueryValue(key string) string {
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    values := ctx.Query[key]
    if len(values) > 0 {
        return values[0]
    }
    return ""
}

func (ctx *Ctx[V]) DefaultQuery(key, defaultValue string) string {
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    values := ctx.Query[key]
    if len(values) > 0 {
        return values[0]
    }
    return defaultValue
}

func (ctx *Ctx[V]) QueryArray(key string) []string {
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    return ctx.Query[key]
}

func (ctx *Ctx[V]) QueryMap() map[string][]string {
    // Thread-safe lazy initialization of query params
    if ctx.Query == nil {
        // Create a new map to avoid race conditions
        query := ctx.Request.URL.Query()
        // Use a single assignment which is atomic for most architectures
        ctx.Query = query
    }
    
    return ctx.Query
}

func (ctx *Ctx[V]) Context() context.Context {
    return ctx.Request.Context()
}

func (ctx *Ctx[V]) Done() {
    if ctx.done {
        return
    }
    ctx.done = true
}

func (ctx *Ctx[V]) IsDone() bool {
    return ctx.done
}

// Returns client IP with proxy awareness (X-Forwarded-For, X-Real-IP fallbacks)
func (ctx *Ctx[V]) ClientIP() string {
    // Assert prerequisites - ensure Request exists
    if ctx.Request == nil {
        // Return a safe default for failure cases
        return "0.0.0.0"
    }
    
    // Fast path - check X-Forwarded-For header first (most common in proxied environments)
    if xForwardedFor := ctx.Request.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
        // Fast path for simple case - single IP without comma
        if !strings.Contains(xForwardedFor, ",") {
            ip := strings.TrimSpace(xForwardedFor)
            if net.ParseIP(ip) != nil {
                return ip
            }
        } else {
            // Multiple IPs - take first valid one
            ips := strings.Split(xForwardedFor, ",")
            if len(ips) > 0 {
                ip := strings.TrimSpace(ips[0])
                if net.ParseIP(ip) != nil {
                    return ip
                }
            }
        }
    }
    
    // Check X-Real-IP next
    if xRealIP := ctx.Request.Header.Get("X-Real-IP"); xRealIP != "" {
        ip := strings.TrimSpace(xRealIP)
        if net.ParseIP(ip) != nil {
            return ip
        }
    }
    
    // Finally, use the direct RemoteAddr (fastest path for direct connections)
    remoteAddr := ctx.Request.RemoteAddr
    if remoteAddr == "" {
        return "0.0.0.0"
    }
    
    // Most RemoteAddr values will have the port
    ip, _, err := net.SplitHostPort(remoteAddr)
    if err != nil {
        // Handle edge case where RemoteAddr has no port
        return remoteAddr
    }
    return ip
}

func (ctx *Ctx[V]) Cookie(name string) (string, error) {
    cookie, err := ctx.Request.Cookie(name)
    if err != nil {
        return "", Wrap(err, ErrInvalidRequest, "cookie not found")
    }
    return cookie.Value, nil
}

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

// Parses request body as JSON into the provided object
func (ctx *Ctx[V]) ShouldBindJSON(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return Wrap(err, ErrInvalidJSON, "failed to read request body")
    }
    
    if len(ctx.Body) == 0 {
        return New(ErrInvalidJSON, "request body is empty")
    }
    
    unmarshalErr := json.Unmarshal(ctx.Body, obj)
    if unmarshalErr != nil {
        return Wrap(unmarshalErr, ErrInvalidJSON, "failed to unmarshal JSON")
    }
    
    return nil
}

// Parses request body as XML into the provided object
func (ctx *Ctx[V]) ShouldBindXML(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return Wrap(err, ErrInvalidRequest, "failed to read request body")
    }
    
    if len(ctx.Body) == 0 {
        return New(ErrInvalidRequest, "request body is empty")
    }
    
    unmarshalErr := xml.Unmarshal(ctx.Body, obj)
    if unmarshalErr != nil {
        return Wrap(unmarshalErr, ErrInvalidRequest, "failed to unmarshal XML")
    }
    
    return nil
}

// Parses form data into the provided object
func (ctx *Ctx[V]) ShouldBindForm(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return Wrap(err, ErrInvalidForm, "failed to read request body")
    }
    
    if err := ctx.Request.ParseForm(); err != nil {
        return Wrap(err, ErrInvalidForm, "failed to parse form")
    }
    
    values := ctx.Request.PostForm
    if mapErr := mapForm(obj, values); mapErr != nil {
        return Wrap(mapErr, ErrInvalidForm, "failed to map form values")
    }
    
    return nil
}

// Parses multipart form data into the provided object
func (ctx *Ctx[V]) ShouldBindMultipartForm(obj interface{}) error {
    err := ctx.NeedBody()
    if err != nil {
        return Wrap(err, ErrInvalidForm, "failed to read request body")
    }
    
    // 32MB max memory
    if err := ctx.Request.ParseMultipartForm(32 << 20); err != nil {
        return Wrap(err, ErrInvalidForm, "failed to parse multipart form")
    }
    
    values := ctx.Request.MultipartForm.Value
    if mapErr := mapForm(obj, values); mapErr != nil {
        return Wrap(mapErr, ErrInvalidForm, "failed to map multipart form values")
    }
    
    return nil
}

// Helper to decode form values into struct using go-playground/form
func mapForm(ptr interface{}, formData url.Values) error {
    return formDecoder.Decode(ptr, formData)
}

// Auto-selects appropriate binding method based on Content-Type
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
        return Newf(ErrInvalidRequest, "unsupported content type: %s", contentType)
    }
}

// Ensures request body is loaded once and cached for reuse
func (ctx *Ctx[V]) NeedBody() error {
    // If we've already read the body, return immediately
    if ctx.hasReadBody {
        return nil
    }
    
    // Assert prerequisites
    if ctx.Request == nil {
        return New(ErrInvalidRequest, "request is nil")
    }
    
    if ctx.ResponseWriter == nil {
        return New(ErrInvalidRequest, "response writer is nil")
    }
    
    // Mark that we've read the body to prevent duplicate reads
    ctx.hasReadBody = true
    ctx.ResponseWriter.CaptureBody = true

    // Apply size limits and read the body
    err := ctx.readBodyWithSizeLimit()
    if err != nil {
        return err
    }
    
    return nil
}

// Reads request body with size limit enforcement
func (ctx *Ctx[V]) readBodyWithSizeLimit() error {
    // Add 1 byte to detect if the body is too large
    limitedReader := io.LimitReader(ctx.Request.Body, GetMaxBodySize()+1)
    body, err := io.ReadAll(limitedReader)
    
    if err != nil {
        ctx.logBodyReadError(err)
        return Wrap(err, ErrInvalidRequest, "failed to read request body")
    }

    // Check if body exceeds maximum size
    if int64(len(body)) > GetMaxBodySize() {
        err := New(ErrInvalidRequest, "request body too large")
        ctx.logBodySizeError(err)
        return err
    }

    // Save the body and create a new reader for the request
    ctx.Request.Body.Close()
    ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
    ctx.Body = body
    
    return nil
}

func (ctx *Ctx[V]) logBodyReadError(err error) {
    octoErr := Wrap(err, ErrInvalidRequest, "failed to read request body")
    if ctx.Request != nil {
        LogErrorWithPathIP(logger, octoErr, ctx.Request.URL.Path, ctx.ClientIP())
    } else {
        LogError(logger, octoErr)
    }
}

func (ctx *Ctx[V]) logBodySizeError(err error) {
    // Skip if logger check is enabled and logger is nil
    if EnableLoggerCheck {
        if logger == nil {
            return
        }
    }
    
    // Create base error event with body size info
    event := logger.Error().Err(err).Int64("maxSize", GetMaxBodySize())
    
    // Add path and IP when available - avoid compound conditional
    if ctx.Request != nil {
        if ctx.Request.URL != nil {
            event = event.Str("path", ctx.Request.URL.Path)
        }
        event = event.Str("ip", ctx.ClientIP())
    }
    
    event.Msg("[octo] request body exceeds maximum allowed size")
}

// Sends standardized error response with proper HTTP status code
func (ctx *Ctx[V]) SendError(code string, err error) {
    if ctx.done {
        return
    }
    
    // Convert string code to ErrorCode
    errorCode := ErrorCode(code)
    
    // Get the API error definition or fall back to unknown error
    apiErrorDef, ok := PredefinedErrors[errorCode]
    if !ok {
        apiErrorDef = PredefinedErrors[ErrUnknown]
        errorCode = ErrUnknown
    }
    
    // Create proper OctoError if not already one
    var octoErr *OctoError
    if err == nil {
        octoErr = New(errorCode, "")
    } else if !errors.As(err, &octoErr) {
        octoErr = Wrap(err, errorCode, "")
    }
    
    // Build the error message and response
    message := octoErr.Message
    if message == "" {
        message = apiErrorDef.Message
    }
    
    if octoErr.Original != nil {
        message = message + ": " + octoErr.Original.Error()
    }
    
    // Log the error with request path and IP context
    if ctx.Request != nil {
        LogErrorWithPathIP(logger, octoErr, ctx.Request.URL.Path, ctx.ClientIP())
    } else {
        LogError(logger, octoErr)
    }
    
    // Create and send response
    result := ctx.createErrorResult(string(errorCode), message)
    ctx.SendJSON(octoErr.StatusCode, result)
}

// Like SendError but allows overriding the HTTP status code
func (ctx *Ctx[V]) SendErrorStatus(statusCode int, code string, err error) {
    if ctx.done {
        return
    }
    
    // Convert string code to ErrorCode
    errorCode := ErrorCode(code)
    
    // Get the API error definition or fall back to unknown error
    apiErrorDef, ok := PredefinedErrors[errorCode]
    if !ok {
        apiErrorDef = PredefinedErrors[ErrUnknown]
        errorCode = ErrUnknown
    }
    
    // Create proper OctoError if not already one
    var octoErr *OctoError
    if err == nil {
        octoErr = New(errorCode, "")
    } else if !errors.As(err, &octoErr) {
        octoErr = Wrap(err, errorCode, "")
    }
    
    // Use provided status code instead of the default one
    octoErr.StatusCode = statusCode
    
    // Build the error message and response
    message := octoErr.Message
    if message == "" {
        message = apiErrorDef.Message
    }
    
    if octoErr.Original != nil {
        message = message + ": " + octoErr.Original.Error()
    }
    
    // Log the error with request path and IP context
    if ctx.Request != nil {
        LogErrorWithPathIP(logger, octoErr, ctx.Request.URL.Path, ctx.ClientIP())
    } else {
        LogError(logger, octoErr)
    }
    
    // Create and send response
    result := ctx.createErrorResult(string(errorCode), message)
    ctx.SendJSON(statusCode, result)
}

// DEPRECATED: Maintained for backward compatibility. Use OctoError instead.
func (ctx *Ctx[V]) buildErrorMessage(baseMessage string, err error) string {
    if err == nil {
        return baseMessage
    }
    
    // Create an OctoError
    octoErr := Wrap(err, ErrUnknown, baseMessage)
    
    // Log it properly with path and IP if available
    if ctx.Request != nil {
        LogErrorWithPathIP(logger, octoErr, ctx.Request.URL.Path, ctx.ClientIP())
    } else {
        LogError(logger, octoErr)
    }
    
    // Return the formatted message
    if octoErr.Message != "" {
        if octoErr.Original != nil {
            return octoErr.Message + ": " + octoErr.Original.Error()
        }
        return octoErr.Message
    }
    
    return baseMessage + ": " + err.Error()
}

// DEPRECATED: Maintained for backward compatibility. Use LogError instead.
func (ctx *Ctx[V]) logError(err error, file string, line int, funcName string) {
    // Create temporary OctoError with the file and line info
    octoErr := &OctoError{
        Original:   err,
        Code:       ErrInternal,
        StatusCode: http.StatusInternalServerError,
        file:       file,
        line:       line,
        function:   funcName,
    }
    
    // Log using the new system
    LogError(logger, octoErr)
}

func (ctx *Ctx[V]) createErrorResult(code string, message string) BaseResult {
    return BaseResult{
        Result:  "error",
        Message: message,
        Token:   code,
        Time:    float64(time.Now().UnixNano()-ctx.StartTime) / 1e9,
    }
}

func (ctx *Ctx[V]) Redirect(status int, url string) {
    if ctx.done {
        return
    }
    http.Redirect(ctx.ResponseWriter, ctx.Request, url, status)
    ctx.Done()
}

func (ctx *Ctx[V]) Send404() {
    if ctx.done {
        return
    }
    // Create a custom error with request path in message
    var err error
    if ctx.Request != nil {
        err = Newf(ErrNotFound, "Route not found: %s", ctx.Request.URL.Path)
    }
    ctx.SendError(string(ErrNotFound), err)
}

func (ctx *Ctx[V]) Send401() {
    if ctx.done {
        return
    }
    // Create a custom error with request path in message
    var err error
    if ctx.Request != nil {
        err = Newf(ErrUnauthorized, "Unauthorized request for: %s", ctx.Request.URL.Path)
    }
    ctx.SendError(string(ErrUnauthorized), err)
}

func (ctx *Ctx[V]) SendInvalidUUID() {
    if ctx.done {
        return
    }
    var message string
    if ctx.Request != nil {
        message = fmt.Sprintf("Invalid UUID in request: %s", ctx.Request.URL.Path)
    } else {
        message = "Invalid UUID"
    }
    ctx.SendError(string(ErrInvalidRequest), New(ErrInvalidRequest, message))
}

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

func (ctx *Ctx[V]) SendData(statusCode int, contentType string, data []byte) error {
    if ctx.done {
        return nil
    }
    
    // Set headers
    ctx.SetHeader("Content-Type", contentType)
    ctx.SetHeader("Content-Length", strconv.Itoa(len(data)))
    ctx.SetStatus(statusCode)
    
    // Write data
    _, err := ctx.ResponseWriter.Write(data)
    if err != nil {
        ctx.logDataWriteError(err)
        // Return error to caller for proper handling
        return err
    }
    
    ctx.Done()
    return nil
}

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

func (ctx *Ctx[V]) File(urlPath string, filePath string) {
    if ctx.done {
        return
    }
    
    if filePath == "" {
        err := New(ErrInternal, "file path is empty")
        ctx.SendError(string(ErrInternal), err)
        return
    }
    
    http.ServeFile(ctx.ResponseWriter, ctx.Request, filePath)
    ctx.Done()
}

func (ctx *Ctx[V]) FileFromFS(urlPath string, fs http.FileSystem, filePath string) {
    if ctx.done {
        return
    }
    
    if fs == nil {
        err := New(ErrInternal, "http.FileSystem is nil")
        ctx.SendError(string(ErrInternal), err)
        return
    }
    
    http.FileServer(fs).ServeHTTP(ctx.ResponseWriter, ctx.Request)
    ctx.Done()
}

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

func (ctx *Ctx[V]) SendString(statusCode int, s string) {
    ctx.SendData(statusCode, "text/plain", []byte(s))
}

// Standard response envelope for all API responses
type BaseResult struct {
    Data    interface{}         `json:"data,omitempty"`
    Time    float64             `json:"time"`
    Result  string              `json:"result"`
    Message string              `json:"message,omitempty"`
    Paging  *octypes.Pagination `json:"paging,omitempty"`
    Token   string              `json:"token,omitempty"`
}
