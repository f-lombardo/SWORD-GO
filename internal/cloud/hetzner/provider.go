package hetzner

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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

func (p *Provider) CreateServer(ctx context.Context, data CreateServerData) (ServerResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if data.PublicIPPollAttempts < 1 {
		return ServerResult{}, errors.New("public IP poll attempts must be greater than zero")
	}

	if data.PublicIPPollIntervalSecs < 1 {
		return ServerResult{}, errors.New("public IP poll interval must be greater than zero")
	}

	location := data.RegionOrLocation()

	resolvedKey, err := p.sshKeys.Resolve(ctx, data.PublicKey, data.APIKey)
	if err != nil {
		return ServerResult{}, err
	}

	payload := map[string]any{
		"name":        data.Name,
		"location":    location,
		"server_type": data.ServerType,
		"image":       data.Image,
		"ssh_keys":    []any{resolvedKey.ID},
		"public_net": map[string]any{
			"ipv4_enabled": true,
			"ipv6_enabled": false,
		},
	}

	res, err := p.client.JSONRequest(ctx, http.MethodPost, "/servers", data.APIKey, payload)
	if err != nil {
		return ServerResult{}, err
	}

	var body struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
		Server struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			DC     struct {
				Location struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"location"`
			} `json:"datacenter"`
			ServerType struct {
				Name string `json:"name"`
			} `json:"server_type"`
		} `json:"server"`
	}

	if err = cloud.DecodeResponse(res, &body); err != nil {
		return ServerResult{}, err
	}

	if res.StatusCode != http.StatusCreated {
		message := cmp.Or(body.Error.Message, body.Message, "Unexpected response from the Hetzner API.")

		return ServerResult{}, fmt.Errorf("Hetzner API error [%d]: %s", res.StatusCode, message)
	}

	if body.Server.ID == 0 || body.Server.Name == "" || body.Server.Status == "" {
		return ServerResult{}, errors.New("Hetzner API response did not include server details")
	}

	publicIP, err := p.waitForPublicIP(ctx, body.Server.ID, data.APIKey, data.PublicIPPollAttempts, data.PublicIPPollIntervalSecs)
	if err != nil {
		return ServerResult{}, err
	}

	finalLocation := cmp.Or(body.Server.DC.Location.Name, body.Server.DC.Location.Description, location)
	serverType := cmp.Or(body.Server.ServerType.Name, data.ServerType)

	return ServerResult{
		ID:           body.Server.ID,
		DropletID:    body.Server.ID,
		ServerID:     body.Server.ID,
		Name:         body.Server.Name,
		Region:       finalLocation,
		Location:     finalLocation,
		Type:         serverType,
		ServerType:   serverType,
		Status:       body.Server.Status,
		PublicIP:     publicIP,
		SSHKeyID:     resolvedKey.ID,
		SSHKeyName:   resolvedKey.Name,
		SSHKeyStatus: resolvedKey.Status,
	}, nil
}

func (p *Provider) waitForPublicIP(ctx context.Context, serverID int, apiKey string, attempts int, intervalSeconds int) (*string, error) {
	ip, err := cloud.WaitFor(ctx, attempts, time.Duration(intervalSeconds)*time.Second, func() (*string, error) {
		res, err := p.client.Request(ctx, http.MethodGet, fmt.Sprintf("/servers/%d", serverID), apiKey, nil, nil)
		if err != nil {
			return nil, err
		}

		var body struct {
			Server struct {
				PublicNet struct {
					IPv4 struct {
						IP string `json:"ip"`
					} `json:"ipv4"`
				} `json:"public_net"`
			} `json:"server"`
		}

		if err = cloud.DecodeResponse(res, &body); err != nil {
			return nil, err
		}

		if body.Server.PublicNet.IPv4.IP != "" {
			publicIP := body.Server.PublicNet.IPv4.IP
			return &publicIP, nil
		}

		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed while waiting for the server public IP: %w", err)
	}

	return ip, nil
}

func (p *Provider) authorizedRequestWithStatusOK(ctx context.Context, method string, path string, apiKey string) (*http.Response, error) {
	return p.client.RequestWithStatusOK(ctx, method, path, apiKey, "Hetzner")
}
