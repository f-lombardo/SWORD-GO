package digitalocean

import (
	"net/http"
)

func (c *Creator) ListAvailableRegions(apiKey string) ([]Region, error) {
	res, err := c.authorizedRequestWithStatusOK(http.MethodGet, "/regions?per_page=200", apiKey, nil)
	if err != nil {
		return nil, err
	}

	var result RegionsResponse
	if err = decodeResponse(res, &result); err != nil {
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

	res, err := c.authorizedRequest(http.MethodGet, "/sizes?per_page=200", apiKey, nil)
	if err != nil {
		return nil, err
	}

	var result SizesResponse
	if err = decodeResponse(res, &result); err != nil {
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
