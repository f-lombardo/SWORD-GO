package hostinger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"sword-go/internal/cloud"
)

const apiBase = "https://developers.hostinger.com/api"

type HTTPDoer = cloud.HTTPDoer

type Provider struct {
	client *cloud.Client
}

func NewProvider(client HTTPDoer) *Provider {
	return &Provider{client: cloud.NewClient(apiBase, client)}
}

func (p *Provider) CreateServer(ctx context.Context, data cloud.CreateServerData) (cloud.ServerResult, error) {
	dataCenterID, err := strconv.Atoi(data.RegionOrLocation())
	if err != nil || dataCenterID < 1 {
		return cloud.ServerResult{}, errors.New("Hostinger data center ID must be a positive number")
	}
	templateID, err := strconv.Atoi(data.Image)
	if err != nil || templateID < 1 {
		return cloud.ServerResult{}, errors.New("Hostinger template ID must be a positive number")
	}
	payload := map[string]any{
		"item_id": data.ServerType,
		"setup": map[string]any{
			"template_id":    templateID,
			"data_center_id": dataCenterID,
			"password":       data.RootPassword,
			"hostname":       data.Name,
			"public_key": map[string]string{
				"name": data.Name + "-key",
				"key":  data.PublicKey,
			},
		},
	}
	res, err := p.client.JSONRequest(ctx, http.MethodPost, "/vps/v1/virtual-machines", data.APIKey, payload)
	if err != nil {
		return cloud.ServerResult{}, err
	}
	defer res.Body.Close()
	var body struct {
		Order struct {
			Status string `json:"status"`
		} `json:"order"`
		VirtualMachine VirtualMachine `json:"virtual_machine"`
		Error          string         `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return cloud.ServerResult{}, err
	}
	if res.StatusCode != http.StatusOK {
		if body.Error == "" {
			body.Error = "unexpected Hostinger API response"
		}
		return cloud.ServerResult{}, fmt.Errorf("Hostinger API error [%d]: %s", res.StatusCode, body.Error)
	}
	if body.VirtualMachine.ID == 0 {
		return cloud.ServerResult{}, errors.New("Hostinger API response did not include a virtual machine")
	}
	var publicIP *string
	if len(body.VirtualMachine.IPv4) > 0 && body.VirtualMachine.IPv4[0].Address != "" {
		publicIP = &body.VirtualMachine.IPv4[0].Address
	}
	return cloud.ServerResult{ID: body.VirtualMachine.ID, ServerID: body.VirtualMachine.ID, Name: body.VirtualMachine.Hostname, Region: strconv.Itoa(body.VirtualMachine.DataCenterID), Location: strconv.Itoa(body.VirtualMachine.DataCenterID), ServerType: body.VirtualMachine.Plan, Type: body.VirtualMachine.Plan, Status: body.VirtualMachine.State, PublicIP: publicIP, SSHKeyStatus: "provided"}, nil
}
