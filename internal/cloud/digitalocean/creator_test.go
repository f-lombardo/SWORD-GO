package digitalocean

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type queueHTTPClient struct {
	requests  []*http.Request
	responses []*http.Response
}

func (c *queueHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	response := c.responses[0]
	c.responses = c.responses[1:]
	return response, nil
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestCreatorUploadsRawSSHKeyAndReturnsDropletDetails(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusCreated, `{"ssh_key":{"id":44,"name":"uploaded-key"}}`),
			jsonResponse(http.StatusAccepted, `{"droplet":{"id":999,"name":"sword-web","region":{"slug":"ams3"},"size_slug":"s-1vcpu-1gb","status":"new"}}`),
			jsonResponse(http.StatusOK, `{"droplet":{"networks":{"v4":[{"type":"public","ip_address":"203.0.113.10"}]}}}`),
		},
	}

	creator := NewCreator(client)
	result, err := creator.Create(context.Background(), CreateDropletData{
		APIKey:                   "test-token",
		Name:                     "sword-web",
		Region:                   "ams3",
		ServerType:               "s-1vcpu-1gb",
		Image:                    "ubuntu-24-04-x64",
		PublicKey:                "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB-example uploaded-key",
		PublicIPPollAttempts:     1,
		PublicIPPollIntervalSecs: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DropletID != 999 {
		t.Fatalf("expected droplet ID 999, got %d", result.DropletID)
	}

	if result.PublicIP == nil || *result.PublicIP != "203.0.113.10" {
		t.Fatalf("expected public IP 203.0.113.10, got %#v", result.PublicIP)
	}

	if result.SSHKeyStatus != "uploaded" {
		t.Fatalf("expected ssh key status uploaded, got %s", result.SSHKeyStatus)
	}

	if len(client.requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.digitalocean.com/v2/account/keys" {
		t.Fatalf("unexpected first URL: %s", got)
	}

	if got := client.requests[1].URL.String(); got != "https://api.digitalocean.com/v2/droplets" {
		t.Fatalf("unexpected second URL: %s", got)
	}

	if got := client.requests[2].URL.String(); got != "https://api.digitalocean.com/v2/droplets/999" {
		t.Fatalf("unexpected third URL: %s", got)
	}
}

func TestCreatorResolvesExistingSSHKeyByName(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{"ssh_keys":[{"id":22,"name":"existing-key","public_key":"ssh-ed25519 AAAA existing-key"}]}`),
			jsonResponse(http.StatusAccepted, `{"droplet":{"id":123,"name":"sword-app","status":"new"}}`),
			jsonResponse(http.StatusOK, `{"droplet":{"networks":{"v4":[{"type":"public","ip_address":"198.51.100.7"}]}}}`),
		},
	}

	creator := NewCreator(client)
	result, err := creator.Create(context.Background(), CreateDropletData{
		APIKey:                   "test-token",
		Name:                     "sword-app",
		Region:                   "nyc1",
		ServerType:               "s-1vcpu-2gb",
		Image:                    "ubuntu-24-04-x64",
		PublicKey:                "existing-key",
		PublicIPPollAttempts:     1,
		PublicIPPollIntervalSecs: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DropletID != 123 {
		t.Fatalf("expected droplet ID 123, got %d", result.DropletID)
	}

	if result.SSHKeyStatus != "existing" {
		t.Fatalf("expected ssh key status existing, got %s", result.SSHKeyStatus)
	}

	if len(client.requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.digitalocean.com/v2/account/keys" {
		t.Fatalf("unexpected first URL: %s", got)
	}
}
