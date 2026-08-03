package log_streamer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogStreamer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log Streamer Suite")
}
