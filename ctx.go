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
    "sync"
    "time"

    // Third-party imports
    "github.com/go-playground/form/v4"
    "github.com/pkg/errors"
    
    // Internal imports
    "github.com/coffyg/octypes"
)

// Global form decoder instance
var formDecoder = form.NewDecoder()

// Buffer pool for JSON encoding and other uses
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// ConnectionType represents the type of HTTP connection
type ConnectionType int

const (
	ConnectionTypeHTTP ConnectionType = iota
	ConnectionTypeSSE
	ConnectionTypeWebSocket
)

// HTTP request context with typed custom data field
type Ctx[V any] struct {
    ResponseWriter   *ResponseWriterWrapper `json:"-"`
    Request          *http.Request          `json:"-"`
    Params           map[string]string      `json:"Params"`
    Query            map[string][]string    `json:"Query"`
    StartTime        int64                  `json:"StartTime"`
    UUID             string                 `json:"UUID"`
    Body             []byte                 `json:"-"`
    Headers          http.Header            `json:"-"`
    Custom           V                      // Generic Custom Field
    ConnectionType   ConnectionType         `json:"-"`
    done             bool
    hasReadBody      bool
    maxBodySizeBytes *int64                 // Per-request body size override (nil = use global)
}

func (ctx *Ctx[V]) Reset() {
    ctx.ResponseWriter = nil
    ctx.Request = nil
    // Clear maps to preserve underlying storage
    for k := range ctx.Params {
        delete(ctx.Params, k)
    }
    for k := range ctx.Query {
        delete(ctx.Query, k)
    }
    ctx.StartTime = 0
    ctx.UUID = ""
    ctx.Body = nil
    ctx.Headers = nil
    var zero V
    ctx.Custom = zero
    ctx.ConnectionType = ConnectionTypeHTTP
    ctx.done = false
    ctx.hasReadBody = false
    ctx.maxBodySizeBytes = nil
}

func (ctx *Ctx[V]) SetHeader(key, value string) {
    // For common headers, we still use Set() to ensure proper canonicalization
    // The optimization comes from using pre-defined string constants
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

func (ctx *Ctx[V]) SetMaxBodySize(bytes int64) {
    ctx.maxBodySizeBytes = &bytes
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
    
    // Get buffer from pool
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        // Don't put large buffers back to pool to avoid memory leaks
        if buf.Cap() <= 64*1024 {
            bufferPool.Put(buf)
        }
    }()
    
    // Create encoder with buffer
    encoder := json.NewEncoder(buf)
    encoder.SetEscapeHTML(false) // Avoid HTML escaping for performance
    
    // Encode to buffer
    err := encoder.Encode(v)
    if err != nil {
        ctx.SendError("err_json_error", err)
        return err
    }
    
    // Remove trailing newline from encoder.Encode
    data := buf.Bytes()
    if len(data) > 0 && data[len(data)-1] == '\n' {
        data = data[:len(data)-1]
    }
    
    // Set headers
    ctx.SetHeader(HeaderContentType, ContentTypeJSON)
    ctx.SetHeader(HeaderContentLength, strconv.Itoa(len(data)))
    ctx.SetStatus(statusCode)

    // Skip body write for HEAD requests (avoids Go HTTP/2 bug #66812)
    if ctx.Request.Method != http.MethodHead {
        _, err = ctx.ResponseWriter.Write(data)
        if err != nil {
            // Simplify nested conditionals
            if EnableLoggerCheck && logger == nil {
                // Skip logging if logger is disabled
            } else {
                // Log with full request context
                wrappedErr := Wrap(err, ErrInternal, "failed to write response")
                LogErrorWithRequest(logger, wrappedErr, ctx.Request, ctx.ClientIP())
            }
            ctx.Done()
            return err
        }
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
    if xForwardedFor := ctx.Request.Header.Get(HeaderXForwardedFor); xForwardedFor != "" {
        // Optimized: find first comma position instead of Contains+Split
        commaPos := strings.IndexByte(xForwardedFor, ',')
        
        var firstIP string
        if commaPos == -1 {
            // No comma - single IP
            firstIP = xForwardedFor
        } else {
            // Multiple IPs - take first one
            firstIP = xForwardedFor[:commaPos]
        }
        
        // Trim spaces only if needed
        if len(firstIP) > 0 && (firstIP[0] == ' ' || firstIP[len(firstIP)-1] == ' ') {
            firstIP = strings.TrimSpace(firstIP)
        }
        
        // Validate IP
        if net.ParseIP(firstIP) != nil {
            return firstIP
        }
    }
    
    // Check X-Real-IP next
    if xRealIP := ctx.Request.Header.Get(HeaderXRealIP); xRealIP != "" {
        // Trim spaces only if needed
        if len(xRealIP) > 0 && (xRealIP[0] == ' ' || xRealIP[len(xRealIP)-1] == ' ') {
            xRealIP = strings.TrimSpace(xRealIP)
        }
        if net.ParseIP(xRealIP) != nil {
            return xRealIP
        }
    }
    
    // Finally, use the direct RemoteAddr (fastest path for direct connections)
    remoteAddr := ctx.Request.RemoteAddr
    if remoteAddr == "" {
        return "0.0.0.0"
    }
    
    // Handle IPv6 addresses
    if len(remoteAddr) > 0 && remoteAddr[0] == '[' {
        // IPv6 with brackets [::1] or [::1]:8080
        if endBracket := strings.IndexByte(remoteAddr, ']'); endBracket != -1 {
            return remoteAddr[1:endBracket]
        }
    }
    
    // For IPv4, find last colon for port separation
    lastColon := strings.LastIndexByte(remoteAddr, ':')
    if lastColon != -1 {
        // Check if this might be IPv6 by counting colons
        colonCount := strings.Count(remoteAddr, ":")
        if colonCount == 1 {
            // IPv4 with port (192.168.1.1:8080)
            return remoteAddr[:lastColon]
        }
        // Multiple colons, likely IPv6 without brackets (::1)
    }
    
    // Return as-is for IPv6 without port or any unparseable format
    return remoteAddr
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
    contentType := ctx.GetHeader(HeaderContentType)
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
    // Use per-request override if set, otherwise use global
    maxSize := GetMaxBodySize()
    if ctx.maxBodySizeBytes != nil {
        maxSize = *ctx.maxBodySizeBytes
    }

    // Add 1 byte to detect if the body is too large
    limitedReader := io.LimitReader(ctx.Request.Body, maxSize+1)
    body, err := io.ReadAll(limitedReader)

    if err != nil {
        ctx.logBodyReadError(err)
        return Wrap(err, ErrInvalidRequest, "failed to read request body")
    }

    // Check if body exceeds maximum size
    if int64(len(body)) > maxSize {
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
        LogErrorWithRequest(logger, octoErr, ctx.Request, ctx.ClientIP())
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
    
    // Log with full request context
    if ctx.Request != nil {
        LogErrorWithRequest(logger, err, ctx.Request, ctx.ClientIP())
    } else {
        LogError(logger, err)
    }
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
    
    // Log the error with full request context including host and URL
    if ctx.Request != nil {
        LogErrorWithRequest(logger, octoErr, ctx.Request, ctx.ClientIP())
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
    
    // Log the error with full request context including host and URL
    if ctx.Request != nil {
        LogErrorWithRequest(logger, octoErr, ctx.Request, ctx.ClientIP())
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
    
    // Log it properly with full request context if available
    if ctx.Request != nil {
        LogErrorWithRequest(logger, octoErr, ctx.Request, ctx.ClientIP())
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

// DEPRECATED: Maintained for backward compatibility. Use LogErrorWithRequest instead.
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
    
    // Log using the new system with request context if available
    if ctx.Request != nil {
        LogErrorWithRequest(logger, octoErr, ctx.Request, ctx.ClientIP())
    } else {
        LogError(logger, octoErr)
    }
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
    // Create a custom error with full URL in message
    var err error
    if ctx.Request != nil {
        // Build full URL
        scheme := "http"
        if ctx.Request.TLS != nil {
            scheme = "https"
        }
        // Check X-Forwarded-Proto header for proxy scenarios
        if proto := ctx.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
            scheme = proto
        }
        host := ctx.Request.Host
        if host == "" {
            host = ctx.Request.URL.Host
        }
        fullURL := fmt.Sprintf("%s://%s%s", scheme, host, ctx.Request.URL.RequestURI())
        err = Newf(ErrNotFound, "Route not found: %s", fullURL)
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
    ctx.SetHeader(HeaderContentType, contentType)
    ctx.SetHeader(HeaderContentLength, strconv.Itoa(len(data)))
    ctx.SetStatus(statusCode)

    // Skip body write for HEAD requests (avoids Go HTTP/2 bug #66812)
    if ctx.Request.Method != http.MethodHead {
        // Write data
        _, err := ctx.ResponseWriter.Write(data)
        if err != nil {
            ctx.logDataWriteError(err)
            // Return error to caller for proper handling
            return err
        }
    }
    
    ctx.Done()
    return nil
}

func (ctx *Ctx[V]) logDataWriteError(err error) {
    if EnableLoggerCheck && logger == nil {
        return
    }
    
    // Log with full request context
    wrappedErr := Wrap(err, ErrInternal, "failed to write response data")
    if ctx.Request != nil {
        LogErrorWithRequest(logger, wrappedErr, ctx.Request, ctx.ClientIP())
    } else {
        LogError(logger, wrappedErr)
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

// DetectConnectionType analyzes the request headers to determine the connection type
func (ctx *Ctx[V]) DetectConnectionType() {
    if ctx.Request == nil {
        ctx.ConnectionType = ConnectionTypeHTTP
        return
    }
    
    // Check for WebSocket upgrade - proper header-based detection
    if strings.EqualFold(ctx.Request.Header.Get("Connection"), "Upgrade") &&
       strings.EqualFold(ctx.Request.Header.Get("Upgrade"), "websocket") {
        ctx.ConnectionType = ConnectionTypeWebSocket
        return
    }
    
    // Check for SSE based on Accept header ONLY
    accept := ctx.Request.Header.Get("Accept")
    if strings.Contains(accept, "text/event-stream") {
        ctx.ConnectionType = ConnectionTypeSSE
        return
    }
    
    ctx.ConnectionType = ConnectionTypeHTTP
}

// IsStreamingConnection returns true if the connection is SSE or WebSocket
func (ctx *Ctx[V]) IsStreamingConnection() bool {
    return ctx.ConnectionType == ConnectionTypeSSE || ctx.ConnectionType == ConnectionTypeWebSocket
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
