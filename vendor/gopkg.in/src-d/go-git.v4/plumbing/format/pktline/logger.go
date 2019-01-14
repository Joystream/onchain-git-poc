package pktline

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var _logger *zerolog.Logger

func getLogger() *zerolog.Logger {
	if _logger == nil {
		l := log.With().Str("component", "go-git/plumbing/format/pktline").Logger()
		_logger = &l
	}
	return _logger
}
