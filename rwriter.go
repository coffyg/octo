package octo

import (
    "bufio"
    "bytes"
    "errors"
    "net"
    "net/http"
)


// Provides response tracking and implements standard http interfaces
type ResponseWriterWrapper struct {
    http.ResponseWriter
    Status      int
    Body        *bytes.Buffer
    CaptureBody bool
}

func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
    var buf *bytes.Buffer
    if !DeferBufferAllocation {
        // Use buffer pool from ctx.go
        buf = bufferPool.Get().(*bytes.Buffer)
        buf.Reset()
    }

    return &ResponseWriterWrapper{
        ResponseWriter: w,
        Status:         http.StatusOK,
        Body:           buf,
        CaptureBody:    false,
    }
}

func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
    w.Status = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}

func (w *ResponseWriterWrapper) Write(data []byte) (int, error) {
    size, err := w.ResponseWriter.Write(data)
    if w.CaptureBody && err == nil {
        // Thread-safe buffer initialization
        if w.Body == nil {
            w.Body = &bytes.Buffer{}
        }
        
        // Copy the data to avoid race conditions if the data is modified concurrently
        dataCopy := make([]byte, len(data))
        copy(dataCopy, data)
        
        _, bufferErr := w.Body.Write(dataCopy)
        if bufferErr != nil && EnableLoggerCheck {
            if logger != nil {
                logger.Error().Err(bufferErr).Msg("[octo] failed to write to response buffer")
            }
        }
    }
    return size, err
}

func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
        return hijacker.Hijack()
    }
    return nil, nil, errors.New("ResponseWriter does not implement http.Hijacker")
}

func (w *ResponseWriterWrapper) Flush() {
    if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
        flusher.Flush()
    }
}

func (w *ResponseWriterWrapper) Push(target string, opts *http.PushOptions) error {
    if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
        return pusher.Push(target, opts)
    }
    return http.ErrNotSupported
}

// Determines if any response data has been written
func (w *ResponseWriterWrapper) Written() bool {
    // Safe check against nil Body first
    if w.Body == nil {
        return false
    }
    return w.Body.Len() > 0
}
