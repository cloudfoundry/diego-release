package sse

import (
	"bytes"
	"io"
	"strconv"
	"time"
)

// Define byte slice constants used in encoding/writing
var (
	idPrefix    = []byte("id: ")
	eventPrefix = []byte("event: ")
	retryPrefix = []byte("retry: ")
	dataPrefix  = []byte("data: ")
	emptyData   = []byte("data")
	newline     = []byte("\n")
)

// Event represents a Server-Sent Event
type Event struct {
	ID    string
	Name  string
	Data  []byte
	Retry time.Duration
}

func (event Event) Encode() string {
	// Make an educated capacity estimate
	capacity := 8 + len(event.ID) + 8 + len(event.Name) + 20
	dataLines := bytes.Count(event.Data, newline) + 1
	capacity += len(event.Data) + (dataLines * 7)

	var buf bytes.Buffer
	buf.Grow(capacity)

	// Use the pre-defined constants
	buf.Write(idPrefix)
	buf.WriteString(event.ID)
	buf.Write(newline)

	buf.Write(eventPrefix)
	buf.WriteString(event.Name)
	buf.Write(newline)

	if event.Retry != 0 {
		buf.Write(retryPrefix)
		buf.WriteString(strconv.FormatInt(int64(event.Retry/1000/1000), 10))
		buf.Write(newline)
	}

	for _, line := range bytes.Split(event.Data, []byte("\n")) {
		if len(line) == 0 {
			buf.Write(emptyData)
		} else {
			buf.Write(dataPrefix)
			buf.Write(line)
		}
		buf.Write(newline)
	}

	buf.Write(newline)

	return buf.String()
}

func (event Event) Write(destination io.Writer) error {
	// Write id
	if _, err := destination.Write(idPrefix); err != nil {
		return err
	}
	if _, err := destination.Write([]byte(event.ID)); err != nil {
		return err
	}
	if _, err := destination.Write(newline); err != nil {
		return err
	}

	// Write event
	if _, err := destination.Write(eventPrefix); err != nil {
		return err
	}
	if _, err := destination.Write([]byte(event.Name)); err != nil {
		return err
	}
	if _, err := destination.Write(newline); err != nil {
		return err
	}

	// Write retry if present
	if event.Retry != 0 {
		if _, err := destination.Write(retryPrefix); err != nil {
			return err
		}

		retryValue := strconv.FormatInt(int64(event.Retry/1000/1000), 10)
		if _, err := destination.Write([]byte(retryValue)); err != nil {
			return err
		}

		if _, err := destination.Write(newline); err != nil {
			return err
		}
	}

	// Write data lines
	for _, line := range bytes.Split(event.Data, []byte("\n")) {
		if len(line) == 0 {
			if _, err := destination.Write(emptyData); err != nil {
				return err
			}
		} else {
			if _, err := destination.Write(dataPrefix); err != nil {
				return err
			}
			if _, err := destination.Write(line); err != nil {
				return err
			}
		}

		if _, err := destination.Write(newline); err != nil {
			return err
		}
	}

	// Final newline
	_, err := destination.Write(newline)
	return err
}
