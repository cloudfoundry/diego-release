package helpers

import (
	"os"
	"time"

	"github.com/onsi/gomega"
)

var DEFAULT_EVENTUALLY_TIMEOUT = 1 * time.Minute
var DEFAULT_CONSISTENTLY_DURATION = 5 * time.Second

func RegisterDefaultTimeouts() {
	var err error
	if os.Getenv("DEFAULT_EVENTUALLY_TIMEOUT") != "" {
		DEFAULT_EVENTUALLY_TIMEOUT, err = time.ParseDuration(os.Getenv("DEFAULT_EVENTUALLY_TIMEOUT"))
		if err != nil {
			panic(err)
		}
	}

	if os.Getenv("DEFAULT_CONSISTENTLY_DURATION") != "" {
		DEFAULT_CONSISTENTLY_DURATION, err = time.ParseDuration(os.Getenv("DEFAULT_CONSISTENTLY_DURATION"))
		if err != nil {
			panic(err)
		}
	}

	gomega.SetDefaultEventuallyTimeout(DEFAULT_EVENTUALLY_TIMEOUT)
	gomega.SetDefaultConsistentlyDuration(DEFAULT_CONSISTENTLY_DURATION)

	// most things hit some component; don't hammer it
	gomega.SetDefaultConsistentlyPollingInterval(100 * time.Millisecond)
	gomega.SetDefaultEventuallyPollingInterval(500 * time.Millisecond)
}
