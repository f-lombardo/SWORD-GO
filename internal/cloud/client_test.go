package cloud

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPDoer struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestClientRequestHeadersAndAuth(t *testing.T) {
	var capturedReq *http.Request

	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := NewClient("https://api.example.com/v1", mock)
	res, err := client.JSONRequest(context.Background(), http.MethodPost, "/test", "secret-token", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer res.Body.Close()

	if capturedReq.URL.String() != "https://api.example.com/v1/test" {
		t.Fatalf("expected URL https://api.example.com/v1/test, got %s", capturedReq.URL.String())
	}
	if capturedReq.Header.Get("Authorization") != "Bearer secret-token" {
		t.Fatalf("expected Bearer secret-token, got %s", capturedReq.Header.Get("Authorization"))
	}
	if capturedReq.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %s", capturedReq.Header.Get("Content-Type"))
	}
}

func TestClientRequestWithStatusOK(t *testing.T) {
	mock := &mockHTTPDoer{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(`error`)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := NewClient("https://api.example.com", mock)
	_, err := client.RequestWithStatusOK(context.Background(), http.MethodGet, "/status", "token", "TestProvider")
	if err == nil {
		t.Fatalf("expected error on HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "API TestProvider: HTTP 500") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
