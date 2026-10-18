package digitalocean

import (
	"context"
	"net/http"
	"testing"
)

func TestProviderListAvailableRegions(t *testing.T) {
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

	provider := NewProvider(client)
	regions, err := provider.ListAvailableRegions(context.Background(), "test-token")
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

func TestProviderListAvailableRegionsHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusUnauthorized, `{"message":"Unable to authenticate you"}`),
		},
	}

	provider := NewProvider(client)
	_, err := provider.ListAvailableRegions(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "API DigitalOcean: HTTP 401"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestProviderListAvailableRegionsInvalidJSON(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `invalid-json`),
		},
	}

	provider := NewProvider(client)
	_, err := provider.ListAvailableRegions(context.Background(), "test-token")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestProviderListSizes(t *testing.T) {
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

	provider := NewProvider(client)
	sizes, err := provider.ListSizes(context.Background(), "test-token")
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

func TestProviderListSizesHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusInternalServerError, `{"message":"Internal Server Error"}`),
		},
	}

	provider := NewProvider(client)
	_, err := provider.ListSizes(context.Background(), "test-token")
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

func TestProviderListAvailableImages(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{
				"images": [
					{
						"id": 101,
						"name": "24.04 (LTS) x64",
						"distribution": "Ubuntu",
						"slug": "ubuntu-24-04-x64",
						"status": "available",
						"public": true
					},
					{
						"id": 102,
						"name": "Retired OS",
						"distribution": "Other",
						"slug": "retired-os",
						"status": "retired",
						"public": false
					},
					{
						"id": 103,
						"name": "12 x64",
						"distribution": "Debian",
						"slug": "debian-12-x64",
						"status": "available",
						"public": true
					}
				]
			}`),
		},
	}

	provider := NewProvider(client)
	images, err := provider.ListAvailableImages(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("expected 2 available images, got %d", len(images))
	}

	if images[0].Slug != "ubuntu-24-04-x64" || images[1].Slug != "debian-12-x64" {
		t.Fatalf("unexpected image slugs: %s, %s", images[0].Slug, images[1].Slug)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.digitalocean.com/v2/images?type=distribution&per_page=200" {
		t.Fatalf("unexpected request URL: %s", got)
	}
}

func TestProviderListAvailableImagesHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusUnauthorized, `{"message":"Unauthorized"}`),
		},
	}

	provider := NewProvider(client)
	_, err := provider.ListAvailableImages(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}

	expectedErr := "API DigitalOcean: HTTP 401"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestImageString(t *testing.T) {
	img := Image{
		ID:           101,
		Slug:         "ubuntu-24-04-x64",
		Distribution: "Ubuntu",
		Name:         "24.04 (LTS) x64",
	}

	expected := "Slug: ubuntu-24-04-x64, Distribution: Ubuntu, Name: 24.04 (LTS) x64"
	if got := img.String(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
