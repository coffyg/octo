package octo

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger
var maxBodySize int64 = 10 * 1024 * 1024

func SetupOctoLogger(l *zerolog.Logger) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	logger = l
}
func SetupOCto(l *zerolog.Logger, mbs int64) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	logger = l
	maxBodySize = mbs
}
