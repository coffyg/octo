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
	status int
	size   int
	body   *bytes.Buffer // Buffer to capture response body
}

// NewResponseWriterWrapper initializes a new ResponseWriterWrapper
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		status:         http.StatusOK, // Default status code
		body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the size and body of the response
func (w *ResponseWriterWrapper) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.size += size
	if err == nil {
		w.body.Write(data) // Capture the response body
	}
	return size, err
}

// Implement http.Hijacker
func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("ResponseWriter does not implement http.Hijacker")
}

// Implement http.Flusher
func (w *ResponseWriterWrapper) Flush() {
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// Implement http.Pusher
func (w *ResponseWriterWrapper) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Removed CloseNotify method as it is deprecated
