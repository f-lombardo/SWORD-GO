package hetzner

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

func TestCreatorUploadsRawSSHKeyAndReturnsServerDetails(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusCreated, `{"ssh_key":{"id":44,"name":"uploaded-key"}}`),
			jsonResponse(http.StatusCreated, `{"server":{"id":999,"name":"sword-web","status":"initializing","server_type":{"name":"cx22"},"datacenter":{"location":{"name":"nbg1"}}}}`),
			jsonResponse(http.StatusOK, `{"server":{"public_net":{"ipv4":{"ip":"203.0.113.10"}}}}`),
		},
	}

	creator := NewCreator(client)
	result, err := creator.Create(context.Background(), CreateServerData{
		APIKey:                   "test-token",
		Name:                     "sword-web",
		Location:                 "nbg1",
		ServerType:               "cx22",
		Image:                    "ubuntu-24.04",
		PublicKey:                "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB-example uploaded-key",
		PublicIPPollAttempts:     1,
		PublicIPPollIntervalSecs: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ServerID != 999 {
		t.Fatalf("expected server ID 999, got %d", result.ServerID)
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

	if got := client.requests[0].URL.String(); got != "https://api.hetzner.cloud/v1/ssh_keys" {
		t.Fatalf("unexpected first URL: %s", got)
	}

	if got := client.requests[1].URL.String(); got != "https://api.hetzner.cloud/v1/servers" {
		t.Fatalf("unexpected second URL: %s", got)
	}

	if got := client.requests[2].URL.String(); got != "https://api.hetzner.cloud/v1/servers/999" {
		t.Fatalf("unexpected third URL: %s", got)
	}
}

func TestCreatorResolvesExistingSSHKeyByName(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{"ssh_keys":[{"id":22,"name":"existing-key","public_key":"ssh-ed25519 AAAA existing-key"}]}`),
			jsonResponse(http.StatusCreated, `{"server":{"id":123,"name":"sword-app","status":"running","server_type":{"name":"cx11"},"datacenter":{"location":{"name":"hel1"}}}}`),
			jsonResponse(http.StatusOK, `{"server":{"public_net":{"ipv4":{"ip":"198.51.100.7"}}}}`),
		},
	}

	creator := NewCreator(client)
	result, err := creator.Create(context.Background(), CreateServerData{
		APIKey:                   "test-token",
		Name:                     "sword-app",
		Location:                 "hel1",
		ServerType:               "cx11",
		Image:                    "ubuntu-24.04",
		PublicKey:                "existing-key",
		PublicIPPollAttempts:     1,
		PublicIPPollIntervalSecs: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ServerID != 123 {
		t.Fatalf("expected server ID 123, got %d", result.ServerID)
	}

	if result.SSHKeyStatus != "existing" {
		t.Fatalf("expected ssh key status existing, got %s", result.SSHKeyStatus)
	}

	if len(client.requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.hetzner.cloud/v1/ssh_keys" {
		t.Fatalf("unexpected first URL: %s", got)
	}
}
