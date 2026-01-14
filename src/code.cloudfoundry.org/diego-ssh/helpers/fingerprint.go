package helpers

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const MD5_FINGERPRINT_LENGTH = 47
const SHA256_FINGERPRINT_LENGTH = 95

func MD5Fingerprint(key ssh.PublicKey) string {
	md5sum := md5.Sum(key.Marshal())
	return colonize(fmt.Sprintf("% x", md5sum))
}

func SHA256Fingerprint(key ssh.PublicKey) string {
	sha256sum := sha256.Sum256(key.Marshal())
	return colonize(fmt.Sprintf("% x", sha256sum))
}

func colonize(s string) string {
	return strings.Replace(s, " ", ":", -1)
}
