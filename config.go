package octo

import (
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger

// Maximum request body size (10MB default)
var bodySizeMaxBytes int64 = 10 * 1024 * 1024

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

func SetupOcto(l *zerolog.Logger, maxBytes int64) {
    zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
    logger = l
    bodySizeMaxBytes = maxBytes
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
