package octo

import (
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger

// BodySizeMaxBytes defines the maximum body size for all requests
var bodySizeMaxBytes int64 = 10 * 1024 * 1024

// DeferBufferAllocation controls buffer allocation in rwriter.go
var DeferBufferAllocation = true

// EnableLoggerCheck guards logger != nil checks in ctx.go
var EnableLoggerCheck = true

// EnableSecurityHeaders adds simple security headers in router.go
var EnableSecurityHeaders = false

// SetupOctoLogger configures the zerolog logger for Octo
func SetupOctoLogger(l *zerolog.Logger) {
    zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
    logger = l
}

// SetupOcto configures the zerolog logger and maximum body size
func SetupOcto(l *zerolog.Logger, maxBytes int64) {
    zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
    logger = l
    bodySizeMaxBytes = maxBytes
}

// GetLogger returns the configured logger instance
func GetLogger() *zerolog.Logger {
    return logger
}

// ChangeMaxBodySize updates the maximum allowed request body size
func ChangeMaxBodySize(maxBytes int64) {
    bodySizeMaxBytes = maxBytes
}

// GetMaxBodySize returns the current maximum body size setting
func GetMaxBodySize() int64 {
    return bodySizeMaxBytes
}
