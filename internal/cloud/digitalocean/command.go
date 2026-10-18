package digitalocean

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sword-go/internal/cloud"
	"time"
)

func (p *Provider) CreateServer(ctx context.Context, data CreateDropletData) (DropletResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if data.PublicIPPollAttempts < 1 {
		return DropletResult{}, errors.New("public IP poll attempts must be greater than zero")
	}

	if data.PublicIPPollIntervalSecs < 1 {
		return DropletResult{}, errors.New("public IP poll interval must be greater than zero")
	}

	region := data.RegionOrLocation()

	resolvedKey, err := p.sshKeys.Resolve(ctx, data.PublicKey, data.APIKey)
	if err != nil {
		return DropletResult{}, err
	}

	payload := map[string]any{
		"name":     data.Name,
		"region":   region,
		"size":     data.ServerType,
		"image":    data.Image,
		"ssh_keys": []any{resolvedKey.ID},
	}

	res, err := p.client.JSONRequest(ctx, http.MethodPost, "/droplets", data.APIKey, payload)
	if err != nil {
		return DropletResult{}, err
	}

	var body struct {
		Message string `json:"message"`
		Droplet struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			SizeSlug string `json:"size_slug"`
			Region   struct {
				Slug string `json:"slug"`
			} `json:"region"`
		} `json:"droplet"`
	}

	if err = cloud.DecodeResponse(res, &body); err != nil {
		return DropletResult{}, err
	}

	if res.StatusCode != http.StatusAccepted {
		message := cmp.Or(body.Message, "Unexpected response from the DigitalOcean API.")

		return DropletResult{}, fmt.Errorf("DigitalOcean API error [%d]: %s", res.StatusCode, message)
	}

	if body.Droplet.ID == 0 || body.Droplet.Name == "" || body.Droplet.Status == "" {
		return DropletResult{}, errors.New("DigitalOcean API response did not include droplet details")
	}

	publicIP, err := p.waitForPublicIP(ctx, body.Droplet.ID, data.APIKey, data.PublicIPPollAttempts, data.PublicIPPollIntervalSecs)
	if err != nil {
		return DropletResult{}, err
	}

	finalRegion := cmp.Or(body.Droplet.Region.Slug, region)
	dropletType := cmp.Or(body.Droplet.SizeSlug, data.ServerType)

	return DropletResult{
		ID:           body.Droplet.ID,
		DropletID:    body.Droplet.ID,
		ServerID:     body.Droplet.ID,
		Name:         body.Droplet.Name,
		Region:       finalRegion,
		Location:     finalRegion,
		Type:         dropletType,
		ServerType:   dropletType,
		Status:       body.Droplet.Status,
		PublicIP:     publicIP,
		SSHKeyID:     resolvedKey.ID,
		SSHKeyName:   resolvedKey.Name,
		SSHKeyStatus: resolvedKey.Status,
	}, nil
}

func (p *Provider) waitForPublicIP(ctx context.Context, dropletID int, apiKey string, attempts int, intervalSeconds int) (*string, error) {
	ip, err := cloud.WaitFor(ctx, attempts, time.Duration(intervalSeconds)*time.Second, func() (*string, error) {
		res, err := p.client.Request(ctx, http.MethodGet, fmt.Sprintf("/droplets/%d", dropletID), apiKey, nil, nil)
		if err != nil {
			return nil, err
		}

		var body struct {
			Droplet struct {
				Networks struct {
					V4 []struct {
						Type      string `json:"type"`
						IPAddress string `json:"ip_address"`
					} `json:"v4"`
				} `json:"networks"`
			} `json:"droplet"`
		}

		if err = cloud.DecodeResponse(res, &body); err != nil {
			return nil, err
		}

		for _, network := range body.Droplet.Networks.V4 {
			if network.Type == "public" && network.IPAddress != "" {
				publicIP := network.IPAddress
				return &publicIP, nil
			}
		}

		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed while waiting for the droplet public IP: %w", err)
	}

	return ip, nil
}
