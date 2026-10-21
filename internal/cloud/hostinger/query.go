package hostinger

import (
	"context"
	"net/http"

	"sword-go/internal/cloud"
)

type DataCenter struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	City      string `json:"city"`
	Continent string `json:"continent"`
}
type Template struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type CatalogItem struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Prices   []CatalogPrice `json:"prices"`
}
type CatalogPrice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	Price      int    `json:"price"`
	Period     int    `json:"period"`
	PeriodUnit string `json:"period_unit"`
}
type IPAddress struct {
	Address string `json:"address"`
}
type VirtualMachine struct {
	ID           int         `json:"id"`
	DataCenterID int         `json:"data_center_id"`
	Plan         string      `json:"plan"`
	Hostname     string      `json:"hostname"`
	State        string      `json:"state"`
	IPv4         []IPAddress `json:"ipv4"`
}

func (p *Provider) ListDataCenters(ctx context.Context, apiKey string) ([]DataCenter, error) {
	var items []DataCenter
	return items, p.get(ctx, "/vps/v1/data-centers", apiKey, &items)
}
func (p *Provider) ListTemplates(ctx context.Context, apiKey string) ([]Template, error) {
	var items []Template
	return items, p.get(ctx, "/vps/v1/templates", apiKey, &items)
}
func (p *Provider) ListVPSCatalog(ctx context.Context, apiKey string) ([]CatalogItem, error) {
	var items []CatalogItem
	return items, p.get(ctx, "/billing/v1/catalog?category=VPS", apiKey, &items)
}

func (p *Provider) get(ctx context.Context, path, apiKey string, target any) error {
	res, err := p.client.RequestWithStatusOK(ctx, http.MethodGet, path, apiKey, "Hostinger")
	if err != nil {
		return err
	}
	return cloud.DecodeResponse(res, target)
}
