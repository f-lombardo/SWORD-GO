package digitalocean

import (
	"fmt"
	"net/http"
	"slices"
	"sword-go/internal/utils"
)

func (c *Creator) ListAvailableRegions(apiKey string) ([]Region, error) {
	res, err := c.authorizedRequestWithStatusOK(http.MethodGet, "/regions?per_page=200", apiKey, nil)
	if err != nil {
		return nil, err
	}

	var result RegionsResponse
	if err = utils.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	var regions []Region

	for _, region := range result.Regions {
		if region.Available {
			regions = append(regions, region)
		}
	}

	return regions, nil
}

func (c *Creator) ListSizes(apiKey string) ([]Size, error) {

	res, err := c.authorizedRequestWithStatusOK(http.MethodGet, "/sizes?per_page=200", apiKey, nil)
	if err != nil {
		return nil, err
	}

	var result SizesResponse
	if err = utils.DecodeResponse(res, &result); err != nil {
		return nil, err
	}
	var sizes []Size

	for _, size := range result.Sizes {
		if size.Available {
			sizes = append(sizes, size)
		}
	}

	return sizes, nil
}

func (region *Region) ListAvailable(sizes []Size) []Size {
	var result []Size

	for _, size := range sizes {
		if slices.Contains(size.Regions, region.Slug) {
			result = append(result, size)
		}
	}

	return result
}

func (size *Size) String() string {
	return fmt.Sprintf("Name: %s, Memory %d MB, VCPUs: %d, Disk %d GB", size.Slug, size.Memory, size.VCPUs, size.Disk)
}
