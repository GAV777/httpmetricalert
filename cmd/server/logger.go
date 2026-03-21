package main

import (
	"time"

	"github.com/rs/zerolog"
)

func setupLogger() {
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}
