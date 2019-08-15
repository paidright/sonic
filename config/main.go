package config

import (
	"os"

	"github.com/davidbanham/required_env"
)

var QUEUE string
var KEWPIE_BACKEND string
var RETRY bool
var SINGLE_SHOT bool

func init() {
	required_env.Ensure(map[string]string{
		"KEWPIE_BACKEND": "",
		"QUEUE":          "",
		"RETRY":          "true",
		"SINGLE_SHOT":    "false",
	})

	KEWPIE_BACKEND = os.Getenv("KEWPIE_BACKEND")
	QUEUE = os.Getenv("QUEUE")
	RETRY = os.Getenv("RETRY") == "true"
	SINGLE_SHOT = os.Getenv("SINGLE_SHOT") == "true"
}
