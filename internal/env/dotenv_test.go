package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `
# Full-line comment
SWORD_GO_HTTP_PORT=8081
SWORD_GO_BASE_URL="http://localhost:8081"
SINGLE_QUOTED='single quoted value'
UNQUOTED_WITH_COMMENT=some_value # inline comment
WITH_EQUALS=file:sword-go.db?cache=shared&mode=rwc
export EXPORTED_VAR=exported_val
EMPTY_VAR=
ESCAPED="hello\nworld\ttab"
`

	got, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"SWORD_GO_HTTP_PORT":    "8081",
		"SWORD_GO_BASE_URL":     "http://localhost:8081",
		"SINGLE_QUOTED":         "single quoted value",
		"UNQUOTED_WITH_COMMENT": "some_value",
		"WITH_EQUALS":           "file:sword-go.db?cache=shared&mode=rwc",
		"EXPORTED_VAR":          "exported_val",
		"EMPTY_VAR":             "",
		"ESCAPED":               "hello\nworld\ttab",
	}

	for k, wantVal := range expected {
		if gotVal, ok := got[k]; !ok {
			t.Errorf("missing key %q", k)
		} else if gotVal != wantVal {
			t.Errorf("for key %q, got %q, want %q", k, gotVal, wantVal)
		}
	}
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	content := `
LOAD_TEST_PORT=9090
LOAD_TEST_OVERWRITE=from_file
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	// Pre-set variable in environment to ensure it is NOT overwritten
	t.Setenv("LOAD_TEST_OVERWRITE", "existing_env_value")

	err := Load(envPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := os.Getenv("LOAD_TEST_PORT"); got != "9090" {
		t.Errorf("expected LOAD_TEST_PORT to be 9090, got %q", got)
	}

	if got := os.Getenv("LOAD_TEST_OVERWRITE"); got != "existing_env_value" {
		t.Errorf("expected existing environment variable to take precedence, got %q", got)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	// Should gracefully succeed without error when file does not exist
	err := Load(filepath.Join(t.TempDir(), "does_not_exist.env"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
}
