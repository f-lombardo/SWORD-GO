package digitalocean

import (
	"context"
	"net/http"
	"testing"
)

func TestCreatorListAvailableRegions(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{
				"regions": [
					{
						"name": "New York 1",
						"slug": "nyc1",
						"available": true,
						"features": ["droplets"]
					},
					{
						"name": "Amsterdam 2",
						"slug": "ams2",
						"available": false,
						"features": []
					},
					{
						"name": "San Francisco 3",
						"slug": "sfo3",
						"available": true,
						"features": ["droplets", "backups"]
					}
				]
			}`),
		},
	}

	creator := NewCreator(client, context.Background())
	regions, err := creator.ListAvailableRegions("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(regions) != 2 {
		t.Fatalf("expected 2 available regions, got %d", len(regions))
	}

	if regions[0].Slug != "nyc1" || regions[1].Slug != "sfo3" {
		t.Fatalf("unexpected region slugs: %s, %s", regions[0].Slug, regions[1].Slug)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.digitalocean.com/v2/regions?per_page=200" {
		t.Fatalf("unexpected request URL: %s", got)
	}

	if got := client.requests[0].Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %s", got)
	}
}

func TestCreatorListAvailableRegionsHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusUnauthorized, `{"message":"Unable to authenticate you"}`),
		},
	}

	creator := NewCreator(client, context.Background())
	_, err := creator.ListAvailableRegions("bad-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "API DigitalOcean: HTTP 401"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestCreatorListAvailableRegionsInvalidJSON(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `invalid-json`),
		},
	}

	creator := NewCreator(client, context.Background())
	_, err := creator.ListAvailableRegions("test-token")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestCreatorListSizes(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{
				"sizes": [
					{
						"slug": "s-1vcpu-1gb",
						"memory": 1024,
						"vcpus": 1,
						"disk": 25,
						"price_monthly": 6.0,
						"price_hourly": 0.00893,
						"regions": ["nyc1", "ams3"],
						"available": true
					},
					{
						"slug": "s-1vcpu-2gb-deprecated",
						"memory": 2048,
						"vcpus": 1,
						"disk": 50,
						"price_monthly": 12.0,
						"price_hourly": 0.01786,
						"regions": ["nyc1"],
						"available": false
					},
					{
						"slug": "s-2vcpu-2gb",
						"memory": 2048,
						"vcpus": 2,
						"disk": 60,
						"price_monthly": 18.0,
						"price_hourly": 0.02679,
						"regions": ["nyc1", "sfo3"],
						"available": true
					}
				]
			}`),
		},
	}

	creator := NewCreator(client, context.Background())
	sizes, err := creator.ListSizes("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sizes) != 2 {
		t.Fatalf("expected 2 available sizes, got %d", len(sizes))
	}

	if sizes[0].Slug != "s-1vcpu-1gb" || sizes[1].Slug != "s-2vcpu-2gb" {
		t.Fatalf("unexpected size slugs: %s, %s", sizes[0].Slug, sizes[1].Slug)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.digitalocean.com/v2/sizes?per_page=200" {
		t.Fatalf("unexpected request URL: %s", got)
	}
}

func TestCreatorListSizesHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusInternalServerError, `{"message":"Internal Server Error"}`),
		},
	}

	creator := NewCreator(client, context.Background())
	_, err := creator.ListSizes("test-token")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}

	expectedErr := "API DigitalOcean: HTTP 500"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestRegionListAvailable(t *testing.T) {
	region := Region{
		Name: "New York 1",
		Slug: "nyc1",
	}

	sizes := []Size{
		{
			Slug:    "s-1vcpu-1gb",
			Regions: []string{"nyc1", "ams3"},
		},
		{
			Slug:    "s-1vcpu-2gb",
			Regions: []string{"lon1", "fra1"},
		},
		{
			Slug:    "s-2vcpu-4gb",
			Regions: []string{"nyc1", "sfo3"},
		},
	}

	available := region.ListAvailable(sizes)
	if len(available) != 2 {
		t.Fatalf("expected 2 available sizes in nyc1, got %d", len(available))
	}

	if available[0].Slug != "s-1vcpu-1gb" || available[1].Slug != "s-2vcpu-4gb" {
		t.Fatalf("unexpected available sizes: %s, %s", available[0].Slug, available[1].Slug)
	}
}

func TestSizeString(t *testing.T) {
	size := Size{
		Slug:   "s-1vcpu-1gb",
		Memory: 1024,
		VCPUs:  1,
		Disk:   25,
	}

	expected := "Name: s-1vcpu-1gb, Memory 1024 MB, VCPUs: 1, Disk 25 GB"
	if got := size.String(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
