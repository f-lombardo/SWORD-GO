package utils

import (
	"crypto/rand"
	"time"
)

const randomCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

func RandomLowerAlphaNumeric(length int) string {
	if length <= 0 {
		return ""
	}

	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		fallback := make([]byte, length)
		for index := range fallback {
			fallback[index] = randomCharset[index%len(randomCharset)]
		}
		return string(fallback)
	}

	out := make([]byte, length)
	for index, value := range randomBytes {
		out[index] = randomCharset[int(value)%len(randomCharset)]
	}

	return string(out)
}

func SecondsFrom(at time.Time) int64 {
	return int64(time.Since(at).Seconds())
}
