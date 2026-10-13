package digitalocean

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const apiURL = "https://api.digitalocean.com/v2/regions"

func ListAvailableRegions(client *http.Client, token string) ([]Region, error) {
	var regions []Region

	nextURL := apiURL + "?per_page=200"

	for nextURL != "" {
		req, err := http.NewRequest(http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creazione richiesta: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("richiesta API: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf(
				"API DigitalOcean: HTTP %d",
				resp.StatusCode,
			)
		}

		var result RegionsResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("decodifica JSON: %w", err)
		}

		for _, region := range result.Regions {
			if region.Available {
				regions = append(regions, region)
			}
		}

		nextURL = result.Links.Pages.Next
	}

	return regions, nil
}
