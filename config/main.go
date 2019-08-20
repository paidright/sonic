package config

import (
	"log"
	"os"
	"time"

	"github.com/davidbanham/required_env"
)

var QUEUE string
var KEWPIE_BACKEND string
var RETRY bool
var SINGLE_SHOT bool
var DIE_IF_IDLE bool
var MAX_IDLE time.Duration

func init() {
	required_env.Ensure(map[string]string{
		"KEWPIE_BACKEND": "",
		"QUEUE":          "",
		"RETRY":          "true",
		"SINGLE_SHOT":    "false",
		"DIE_IF_IDLE":    "false",
		"MAX_IDLE":       "30s",
	})

	KEWPIE_BACKEND = os.Getenv("KEWPIE_BACKEND")
	QUEUE = os.Getenv("QUEUE")
	RETRY = os.Getenv("RETRY") == "true"
	SINGLE_SHOT = os.Getenv("SINGLE_SHOT") == "true"
	DIE_IF_IDLE = os.Getenv("DIE_IF_IDLE") == "true"

	parsed, err := time.ParseDuration(os.Getenv("MAX_IDLE"))
	if err != nil {
		log.Fatal(err)
	}
	MAX_IDLE = parsed
}
