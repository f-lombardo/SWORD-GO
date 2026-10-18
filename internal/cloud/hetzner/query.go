package hetzner

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sword-go/internal/cloud"
)

// ListAvailableLocations returns all available locations in Hetzner Cloud.
func (c *Creator) ListAvailableLocations(ctx context.Context, apiKey string) ([]Location, error) {
	res, err := c.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/locations?per_page=50", apiKey)
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
func (c *Creator) ListServerTypes(ctx context.Context, apiKey string) ([]ServerType, error) {
	res, err := c.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/server_types?per_page=50", apiKey)
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
func (c *Creator) ListAvailableImages(ctx context.Context, apiKey string) ([]Image, error) {
	res, err := c.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/images?type=system&per_page=100", apiKey)
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
