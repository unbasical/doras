package utils

import (
	"crypto/sha256"
	"fmt"
)

func CalcSha256Hex(b []byte) string {
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash)
}
