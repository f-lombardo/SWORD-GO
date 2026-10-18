package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// HTTPDoer abstracts executing HTTP requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client handles authenticated HTTP requests for cloud provider APIs.
type Client struct {
	BaseURL string
	HTTP    HTTPDoer
}

// NewClient initializes a new cloud API client.
func NewClient(baseURL string, httpDoer HTTPDoer) *Client {
	if httpDoer == nil {
		httpDoer = http.DefaultClient
	}
	return &Client{
		BaseURL: baseURL,
		HTTP:    httpDoer,
	}
}

// Request executes an authorized HTTP request against the provider base URL.
func (c *Client) Request(ctx context.Context, method string, path string, apiKey string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	return res, nil
}

// JSONRequest serializes a payload to JSON and executes an authorized request.
func (c *Client) JSONRequest(ctx context.Context, method string, path string, apiKey string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, errors.New("failed to encode API request payload")
		}
		body = bytes.NewReader(encoded)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	return c.Request(ctx, method, path, apiKey, body, headers)
}

// RequestWithStatusOK performs a request and ensures a 200 OK status code.
func (c *Client) RequestWithStatusOK(ctx context.Context, method string, path string, apiKey string, providerName string) (*http.Response, error) {
	res, err := c.Request(ctx, method, path, apiKey, nil, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("API %s: HTTP %d", providerName, res.StatusCode)
	}

	return res, nil
}
