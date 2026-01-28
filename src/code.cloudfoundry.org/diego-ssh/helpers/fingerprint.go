package helpers

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const MD5_FINGERPRINT_LENGTH = 47
const SHA1_FINGERPRINT_LENGTH = 59
const SHA256_FINGERPRINT_LENGTH = 44 //unpadded base64

func MD5Fingerprint(key ssh.PublicKey) string {
	md5sum := md5.Sum(key.Marshal())
	return colonize(fmt.Sprintf("% x", md5sum))
}

func SHA256Fingerprint(key ssh.PublicKey) string {
	value := ssh.FingerprintSHA256(key)
	return strings.TrimPrefix(value, "SHA256:")
}

func SHA1Fingerprint(key ssh.PublicKey) string {
	sha1sum := sha1.Sum(key.Marshal())
	return colonize(fmt.Sprintf("% x", sha1sum))
}

func colonize(s string) string {
	return strings.Replace(s, " ", ":", -1)
}
