package hetzner

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sword-go/internal/cloud"
)

// Pagination

type Pagination struct {
	NextPage *int `json:"next_page"`
}

type Meta struct {
	Pagination Pagination `json:"pagination"`
}

type Location struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	City        string `json:"city"`
	Country     string `json:"country"`
	NetworkZone string `json:"network_zone"`
}

type ServerTypeLocation struct {
	Name        string `json:"name"`
	Available   bool   `json:"available"`
	Recommended bool   `json:"recommended"`
}

type Money struct {
	Net   string `json:"net"`
	Gross string `json:"gross"`
}

type Price struct {
	Location     string `json:"location"`
	PriceHourly  Money  `json:"price_hourly"`
	PriceMonthly Money  `json:"price_monthly"`
}

type ServerType struct {
	ID           int                  `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Cores        int                  `json:"cores"`
	Memory       float64              `json:"memory"`
	Disk         int                  `json:"disk"`
	Architecture string               `json:"architecture"`
	CPUType      string               `json:"cpu_type"`
	Deprecated   bool                 `json:"deprecated"`
	Locations    []ServerTypeLocation `json:"locations"`
	Prices       []Price              `json:"prices"`
}

type LocationsResponse struct {
	Locations []Location `json:"locations"`
	Meta      Meta       `json:"meta"`
}

type ServerTypesResponse struct {
	ServerTypes []ServerType `json:"server_types"`
	Meta        Meta         `json:"meta"`
}

type Image struct {
	ID           int     `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Name         *string `json:"name"`
	Description  string  `json:"description"`
	OSFlavor     string  `json:"os_flavor"`
	OSVersion    string  `json:"os_version"`
	Architecture string  `json:"architecture"`
	Deprecated   *string `json:"deprecated"`
}

type ImagesResponse struct {
	Images []Image `json:"images"`
	Meta   Meta    `json:"meta"`
}

// ListAvailableLocations returns all available locations in Hetzner Cloud.
func (p *Provider) ListAvailableLocations(ctx context.Context, apiKey string) ([]Location, error) {
	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/locations?per_page=50", apiKey)
	if err != nil {
		return nil, err
	}

	var result LocationsResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	return result.Locations, nil
}

// ListServerTypes returns all non-deprecated server types.
func (p *Provider) ListServerTypes(ctx context.Context, apiKey string) ([]ServerType, error) {
	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/server_types?per_page=50", apiKey)
	if err != nil {
		return nil, err
	}

	var result ServerTypesResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	var serverTypes []ServerType

	for _, serverType := range result.ServerTypes {
		if !serverType.Deprecated {
			serverTypes = append(serverTypes, serverType)
		}
	}

	return serverTypes, nil
}

func (location *Location) ListAvailable(serverTypes []ServerType) []ServerType {
	var result []ServerType

	for _, st := range serverTypes {
		if slices.ContainsFunc(st.Locations, func(loc ServerTypeLocation) bool {
			return loc.Name == location.Name && loc.Available
		}) {
			result = append(result, st)
		}
	}

	return result
}

func (st *ServerType) String() string {
	return fmt.Sprintf("Name: %s, Memory %g GB, VCPUs: %d, Disk %d GB", st.Name, st.Memory, st.Cores, st.Disk)
}

// ListAvailableImages returns all available system images.
func (p *Provider) ListAvailableImages(ctx context.Context, apiKey string) ([]Image, error) {
	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/images?type=system&per_page=100", apiKey)
	if err != nil {
		return nil, err
	}

	var result ImagesResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	var images []Image
	for _, img := range result.Images {
		if img.Status == "available" {
			images = append(images, img)
		}
	}

	return images, nil
}

func (img *Image) String() string {
	name := ""
	if img.Name != nil {
		name = *img.Name
	}
	if name == "" {
		name = fmt.Sprintf("%d", img.ID)
	}
	if img.Architecture != "" {
		return fmt.Sprintf("Name: %s, Architecture: %s, Description: %s", name, img.Architecture, img.Description)
	}
	return fmt.Sprintf("Name: %s, Description: %s", name, img.Description)
}
