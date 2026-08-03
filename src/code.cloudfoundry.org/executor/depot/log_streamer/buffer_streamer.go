package log_streamer

import "io"

type bufferStreamer struct {
	stdout     io.Writer
	stderr     io.Writer
	sourceName string
}

func NewBufferStreamer(stdout, stderr io.Writer) LogStreamer {
	return &bufferStreamer{
		stdout:     stdout,
		stderr:     stderr,
		sourceName: DefaultLogSource,
	}
}

func (bs *bufferStreamer) Stdout() io.Writer {
	return bs.stdout
}

func (bs *bufferStreamer) Stderr() io.Writer {
	return bs.stderr
}

func (bs *bufferStreamer) UpdateTags(tags map[string]string) {

}

func (bs *bufferStreamer) Flush() {
}

func (bs *bufferStreamer) WithSource(sourceName string) LogStreamer {
	bs.sourceName = sourceName
	return bs
}

func (bs *bufferStreamer) SourceName() string {
	return bs.sourceName
}

func (bs *bufferStreamer) Stop() {}
