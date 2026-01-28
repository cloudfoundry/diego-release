package helpers

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
)

func HexValueForByteArray(algorithm string, content []byte) (string, error) {
	hash, err := getHash(algorithm)
	if err != nil {
		return "", err
	}
	hash.Write([]byte(content))
	return fmt.Sprintf(`"%x"`, hash.Sum(nil)), nil
}

func getHash(algorithm string) (hash.Hash, error) {
	switch algorithm {
	case "md5":
		return md5.New(), nil
	case "sha1":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	default:
		return nil, fmt.Errorf("%s not valid", algorithm)
	}
}
