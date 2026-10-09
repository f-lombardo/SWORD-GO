package servers

import (
	"crypto/rand"
	"fmt"
	"strings"
)

var adjectives = []string{"silent", "bold", "frozen", "rapid", "amber", "lucky", "north", "solar"}
var nouns = []string{"falcon", "anchor", "forest", "bridge", "planet", "river", "needle", "harbor"}

func GenerateName() (name string, hostname string, err error) {
	adj, err := randomFrom(adjectives)
	if err != nil {
		return "", "", err
	}

	noun, err := randomFrom(nouns)
	if err != nil {
		return "", "", err
	}

	suffix, err := randomNumber(100, 999)
	if err != nil {
		return "", "", err
	}

	name = fmt.Sprintf("%s %s %d", upperFirst(adj), upperFirst(noun), suffix)
	hostname = fmt.Sprintf("%s-%s-%d", adj, noun, suffix)

	return name, hostname, nil
}

func upperFirst(value string) string {
	if value == "" {
		return value
	}

	return strings.ToUpper(value[:1]) + value[1:]
}

func randomFrom(values []string) (string, error) {
	index, err := randomNumber(0, len(values)-1)
	if err != nil {
		return "", err
	}
	return values[index], nil
}

func randomNumber(min int, max int) (int, error) {
	if max < min {
		return 0, fmt.Errorf("invalid range")
	}

	width := max - min + 1
	buffer := make([]byte, 1)
	if _, err := rand.Read(buffer); err != nil {
		return 0, err
	}
	return min + int(buffer[0])%width, nil
}
