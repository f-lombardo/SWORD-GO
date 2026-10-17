package utils

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

func DecodeResponse(response *http.Response, out any) error {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, out); err != nil {
		return errors.New("Hetzner API returned an invalid JSON response")
	}

	return nil
}
