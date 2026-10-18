package utils

import (
	"crypto/rand"
	"os"
	"strings"
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

func SecondsFrom(startingTime time.Time) int64 {
	return int64(time.Since(startingTime).Seconds())
}

func ResolveSecret(commandLineSecret string, secretPath string, envName string) string {
	if strings.TrimSpace(commandLineSecret) != "" {
		return strings.TrimSpace(commandLineSecret)
	}

	secretFromFile := ReadSecretFromPath(secretPath)
	if secretFromFile != "" {
		return secretFromFile
	}

	envSecret := strings.TrimSpace(os.Getenv(envName))
	if envSecret != "" {
		return envSecret
	}

	return ""
}

func ReadSecretFromPath(secretPath string) string {
	info, err := os.Stat(secretPath)
	if err != nil || info.IsDir() {
		return ""
	}

	content, err := os.ReadFile(secretPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}
