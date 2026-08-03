package cacheddownloader

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

func HexValue(algorithm, content string) (string, error) {
	return HexValueForByteArray(algorithm, []byte(content))
}

func HexValueForByteArray(algorithm string, content []byte) (string, error) {
	validator, err := NewHashValidator(algorithm)
	if err != nil {
		return "", err
	}
	validator.hash.Write([]byte(content))
	return fmt.Sprintf(`"%x"`, validator.hash.Sum(nil)), nil
}

type ChecksumFailedError struct {
	msg      string
	expected string
	received string
}

func NewChecksumFailedError(msg, expected, received string) error {
	return &ChecksumFailedError{
		msg:      msg,
		expected: expected,
		received: received,
	}
}

func (e *ChecksumFailedError) Error() string {
	return fmt.Sprintf("Checksum failed: '%s', expected '%s', got '%s'",
		e.msg,
		e.expected,
		e.received,
	)
}

type hashValidator struct {
	algorithm string
	hash      hash.Hash
}

func NewHashValidator(algorithm string) (*hashValidator, error) {
	var hash hash.Hash
	switch algorithm {
	case "md5":
		hash = md5.New()
	case "sha1":
		hash = sha1.New()
	case "sha256":
		hash = sha256.New()
	default:
		return nil, NewChecksumFailedError("algorithm invalid", "[md5, sha1, sha256]", algorithm)
	}
	return &hashValidator{
		algorithm,
		hash,
	}, nil
}

func (v hashValidator) Validate(checksumValue string) error {
	byteValue, ok := v.convertToChecksum(checksumValue)

	if !ok {
		return NewChecksumFailedError("checksum missing or invalid", "a valid checksum", checksumValue)
	}

	if !bytes.Equal(byteValue, v.hash.Sum(nil)) {
		return NewChecksumFailedError("checksum mismatch", checksumValue, fmt.Sprintf(`"%x"`, v.hash.Sum(nil)))
	}
	return nil
}

func (v hashValidator) expectedLength() (int, bool) {
	switch v.algorithm {
	case "md5":
		return 32, true
	case "sha1":
		return 40, true
	case "sha256":
		return 64, true
	}
	return -1, false
}

func (v hashValidator) convertToChecksum(checksumValue string) ([]byte, bool) {
	checksumValue = strings.Trim(checksumValue, `"`)

	length, ok := v.expectedLength()
	if !ok || len(checksumValue) != length {
		return nil, false
	}

	c, err := hex.DecodeString(checksumValue)
	if err != nil {
		return nil, false
	}

	return c, true
}
