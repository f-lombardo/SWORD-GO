package env

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// Load reads the specified files (or ".env" by default if none are provided)
// and sets any environment variables that are not already set in the current process.
// If a file does not exist, it is ignored without returning an error.
func Load(filenames ...string) error {
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}

	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}

		vars, err := Parse(f)
		_ = f.Close()
		if err != nil {
			return err
		}

		for key, val := range vars {
			if _, exists := os.LookupEnv(key); !exists {
				if err := os.Setenv(key, val); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Parse reads KEY=VALUE pairs from an io.Reader and returns them as a map.
func Parse(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Support "export KEY=VALUE" syntax
		if after, ok := strings.CutPrefix(line, "export "); ok {
			line = strings.TrimSpace(after)
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}

		val := strings.TrimSpace(parts[1])

		// Handle quoted values or unquoted values with inline comments
		if len(val) >= 2 && ((strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
			(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`))) {
			isDouble := strings.HasPrefix(val, `"`)
			val = val[1 : len(val)-1]
			if isDouble {
				val = unescape(val)
			}
		} else {
			// Strip unquoted inline comments preceded by whitespace
			if idx := strings.Index(val, " #"); idx != -1 {
				val = strings.TrimSpace(val[:idx])
			}
		}

		result[key] = val
	}

	return result, scanner.Err()
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}
