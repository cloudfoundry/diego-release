package fake_log_streamer

import "github.com/onsi/gomega/gbytes"

func NewFakeLogStreamer() *FakeLogStreamer {
	fakeStreamer := new(FakeLogStreamer)

	stdoutBuffer := gbytes.NewBuffer()
	stderrBuffer := gbytes.NewBuffer()
	fakeStreamer.StdoutReturns(stdoutBuffer)
	fakeStreamer.StderrReturns(stderrBuffer)
	fakeStreamer.WithSourceReturns(fakeStreamer)

	return fakeStreamer
}
