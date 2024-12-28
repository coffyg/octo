package octo

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger

// Max body size for all requests
var maxBodySize int64 = 10 * 1024 * 1024

// 1) Defer buffer allocation in rwriter.go
var DeferBufferAllocation = true

// 2) Check logger != nil in ctx.go (guard logging statements)
var EnableLoggerCheck = true

// 3) Add simple security headers in router.go
var EnableSecurityHeaders = false

func SetupOctoLogger(l *zerolog.Logger) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger = l
}

func SetupOCto(l *zerolog.Logger, mbs int64) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger = l
	maxBodySize = mbs
}

func GetLogger() *zerolog.Logger {
	return logger
}

func ChangeMaxBodySize(mbs int64) {
	maxBodySize = mbs
}

func GetMaxBodySize() int64 {
	return maxBodySize
}
