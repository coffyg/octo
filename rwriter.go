package octo

import (
    "bufio"
    "bytes"
    "errors"
    "net"
    "net/http"
)

// ResponseWriterWrapper wraps http.ResponseWriter and captures response data
type ResponseWriterWrapper struct {
    http.ResponseWriter
    Status      int
    Body        *bytes.Buffer // Buffer to capture response body
    CaptureBody bool
}

// NewResponseWriterWrapper initializes a new ResponseWriterWrapper
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
    // If DeferBufferAllocation = false, allocate immediately
    var buf *bytes.Buffer
    if !DeferBufferAllocation {
        buf = &bytes.Buffer{}
    }

    return &ResponseWriterWrapper{
        ResponseWriter: w,
        Status:         http.StatusOK, // Default status code
        Body:           buf,           // can be nil if defer-alloc is true
        CaptureBody:    false,
    }
}

// WriteHeader captures the status code and passes it to the underlying ResponseWriter
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
    w.Status = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response data and passes it to the underlying ResponseWriter
func (w *ResponseWriterWrapper) Write(data []byte) (int, error) {
    size, err := w.ResponseWriter.Write(data)
    if w.CaptureBody && err == nil {
        // Only allocate buffer if we need it
        if w.Body == nil {
            w.Body = &bytes.Buffer{}
        }
        _, bufferErr := w.Body.Write(data)
        if bufferErr != nil && EnableLoggerCheck {
            if logger != nil {
                logger.Error().Err(bufferErr).Msg("[octo] failed to write to response buffer")
            }
        }
    }
    return size, err
}

// Hijack implements the http.Hijacker interface
func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
        return hijacker.Hijack()
    }
    return nil, nil, errors.New("ResponseWriter does not implement http.Hijacker")
}

// Flush implements the http.Flusher interface
func (w *ResponseWriterWrapper) Flush() {
    if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
        flusher.Flush()
    }
}

// Push implements the http.Pusher interface for HTTP/2 server push
func (w *ResponseWriterWrapper) Push(target string, opts *http.PushOptions) error {
    if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
        return pusher.Push(target, opts)
    }
    return http.ErrNotSupported
}

// Written returns true if the response writer has already written data
func (w *ResponseWriterWrapper) Written() bool {
    return w.Body != nil && w.Body.Len() > 0
}
