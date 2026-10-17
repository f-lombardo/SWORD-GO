package cloud

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIsRawPublicKey(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test@example.com", true},
		{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test", true},
		{"ecdsa-sha2-nistp256 AAAAE2VjZHNh test", true},
		{"sk-ssh-ed25519@openssh.com AAAAG test", true},
		{"my-existing-key-name", false},
		{"12345", false},
	}

	for _, tc := range tests {
		if got := IsRawPublicKey(tc.input); got != tc.expected {
			t.Errorf("IsRawPublicKey(%q) = %v; want %v", tc.input, got, tc.expected)
		}
	}
}

func TestSSHKeyServiceResolveExistingKeyByName(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			body := `{"ssh_keys":[{"id":101,"name":"production-key","public_key":"ssh-ed25519 AAAAB test"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := NewClient("https://api.example.com", mock)
	svc := NewSSHKeyService(client, "/keys")

	resolved, err := svc.Resolve(context.Background(), "production-key", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Status != "existing" {
		t.Fatalf("expected status 'existing', got %s", resolved.Status)
	}
	if resolved.ID != 101 {
		t.Fatalf("expected ID 101, got %v", resolved.ID)
	}
	if resolved.Name == nil || *resolved.Name != "production-key" {
		t.Fatalf("expected name 'production-key', got %#v", resolved.Name)
	}
}

func TestSSHKeyServiceResolveProvidedString(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			body := `{"ssh_keys":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := NewClient("https://api.example.com", mock)
	svc := NewSSHKeyService(client, "/keys")

	resolved, err := svc.Resolve(context.Background(), "unknown-key", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Status != "provided" {
		t.Fatalf("expected status 'provided', got %s", resolved.Status)
	}
	if resolved.ID != "unknown-key" {
		t.Fatalf("expected ID 'unknown-key', got %v", resolved.ID)
	}
}
