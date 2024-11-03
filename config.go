package octo

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger

func SetupOctoLogger(l *zerolog.Logger) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	logger = l
}
