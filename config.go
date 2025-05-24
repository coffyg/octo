package octo

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger

// Maximum request body size (10MB default)
var bodySizeMaxBytes int64 = 10 * 1024 * 1024

// Maximum header size in bytes (1MB default)
// Set to a larger value to prevent 431 Request Header Fields Too Large errors
var headerSizeMaxBytes int64 = 1024 * 1024

// When true, buffer allocation in rwriter.go is deferred until needed
var DeferBufferAllocation = true

// Guards against nil logger access
var EnableLoggerCheck = true

// When true, adds standard security headers to all responses
var EnableSecurityHeaders = false

func SetupOctoLogger(l *zerolog.Logger) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger = l
}

func SetupOcto(l *zerolog.Logger, maxBodyBytes int64, maxHeaderBytes ...int64) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger = l
	bodySizeMaxBytes = maxBodyBytes

	// Optional header size parameter
	if len(maxHeaderBytes) > 0 {
		headerSizeMaxBytes = maxHeaderBytes[0]
	}
}

func GetLogger() *zerolog.Logger {
	return logger
}

func ChangeMaxBodySize(maxBytes int64) {
	bodySizeMaxBytes = maxBytes
}

func GetMaxBodySize() int64 {
	return bodySizeMaxBytes
}

func ChangeMaxHeaderSize(maxBytes int64) {
	headerSizeMaxBytes = maxBytes
}

func GetMaxHeaderSize() int64 {
	return headerSizeMaxBytes
}

// NewHTTPServer creates a new http.Server with configured header and body size limits
// This helps prevent 431 Request Header Fields Too Large errors
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		MaxHeaderBytes: int(headerSizeMaxBytes),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
	}
}

// NewHTTPServerWithConfig creates a new http.Server with custom configuration
func NewHTTPServerWithConfig(addr string, handler http.Handler, readTimeout, writeTimeout, idleTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		MaxHeaderBytes: int(headerSizeMaxBytes),
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		IdleTimeout:    idleTimeout,
	}
}
