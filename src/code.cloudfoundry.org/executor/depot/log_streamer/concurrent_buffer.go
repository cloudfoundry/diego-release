package log_streamer

import (
	"bytes"
	"sync"
)

type ConcurrentBuffer struct {
	Buffer *bytes.Buffer
	*sync.Mutex
}

func NewConcurrentBuffer(payload *bytes.Buffer) *ConcurrentBuffer {
	if payload == nil {
		return nil
	}
	return &ConcurrentBuffer{payload, &sync.Mutex{}}
}

func (b *ConcurrentBuffer) Write(buf []byte) (int, error) {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()
	return b.Buffer.Write(buf)
}

func (b *ConcurrentBuffer) Read(buf []byte) (int, error) {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()
	return b.Buffer.Read(buf)
}

func (b *ConcurrentBuffer) Reset() {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()
	b.Buffer.Reset()
}
