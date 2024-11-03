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
	Status int
	size   int
	Body   *bytes.Buffer // Buffer to capture response body
}

// NewResponseWriterWrapper initializes a new ResponseWriterWrapper
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		Status:         http.StatusOK, // Default status code
		Body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the size and body of the response
func (w *ResponseWriterWrapper) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.size += size
	if err == nil {
		w.Body.Write(data) // Capture the response body
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
