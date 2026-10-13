package digitalocean

import (
	"fmt"
	"net/http"
)

func (c *Creator) ListAvailableRegions(apiKey string) ([]Region, error) {
	res, err := c.authorizedRequest(c.ctx, http.MethodGet, "/regions?per_page=200", apiKey, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf(
			"API DigitalOcean: HTTP %d",
			res.StatusCode,
		)
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
