package octo

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

// ResponseWriterWrapper wraps http.ResponseWriter and implements necessary interfaces
type ResponseWriterWrapper struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader captures the status code
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the size of the response
func (w *ResponseWriterWrapper) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.size += size
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

// Implement CloseNotifier (deprecated but sometimes used)
func (w *ResponseWriterWrapper) CloseNotify() <-chan bool {
	if cn, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}
