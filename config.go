package octo

import "github.com/rs/zerolog"

var logger *zerolog.Logger

func SetupOctoLogger(l *zerolog.Logger) {
	logger = l
}
