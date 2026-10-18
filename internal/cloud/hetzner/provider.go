package hetzner

import (
	"context"
	"net/http"
	"sword-go/internal/cloud"
)

const apiBase = "https://api.hetzner.cloud/v1"

type HTTPDoer = cloud.HTTPDoer

type Provider struct {
	client  *cloud.Client
	sshKeys *cloud.SSHKeyService
}

func NewProvider(client HTTPDoer) *Provider {
	cloudClient := cloud.NewClient(apiBase, client)
	return &Provider{
		client:  cloudClient,
		sshKeys: cloud.NewSSHKeyService(cloudClient, "/ssh_keys"),
	}
}

func (p *Provider) authorizedRequestWithStatusOK(ctx context.Context, method string, path string, apiKey string) (*http.Response, error) {
	return p.client.RequestWithStatusOK(ctx, method, path, apiKey, "Hetzner")
}
