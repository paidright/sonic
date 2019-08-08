package config

import (
	"fmt"
	"os"

	"github.com/davidbanham/required_env"
)

var QUEUE string
var KEWPIE_BACKEND string
var RETRY bool

func init() {
	required_env.Ensure(map[string]string{
		"KEWPIE_BACKEND": "",
		"QUEUE":          "",
		"RETRY":          "true",
	})

	KEWPIE_BACKEND = os.Getenv("KEWPIE_BACKEND")
	QUEUE = os.Getenv("QUEUE")
	RETRY = os.Getenv("RETRY") == "true"

	fmt.Printf("INFO listening on queue: %s \n", QUEUE)
}
