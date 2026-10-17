package hetzner

import (
	"context"
	"net/http"
	"testing"
)

func TestCreatorListAvailableLocations(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{
				"locations": [
					{
						"id": 1,
						"name": "fsn1",
						"description": "Falkenstein DC Park 1",
						"country": "DE",
						"city": "Falkenstein",
						"network_zone": "eu-central"
					},
					{
						"id": 2,
						"name": "nbg1",
						"description": "Nuremberg DC Park 1",
						"country": "DE",
						"city": "Nuremberg",
						"network_zone": "eu-central"
					}
				]
			}`),
		},
	}

	creator := NewCreator(client, context.Background())
	locations, err := creator.ListAvailableLocations("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locations))
	}

	if locations[0].Name != "fsn1" || locations[1].Name != "nbg1" {
		t.Fatalf("unexpected location names: %s, %s", locations[0].Name, locations[1].Name)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.hetzner.cloud/v1/locations?per_page=50" {
		t.Fatalf("unexpected request URL: %s", got)
	}

	if got := client.requests[0].Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %s", got)
	}
}

func TestCreatorListServerTypesFiltersDeprecated(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusOK, `{
				"server_types": [
					{
						"id": 1,
						"name": "cx11",
						"description": "CX11",
						"cores": 1,
						"memory": 2,
						"disk": 20,
						"deprecated": true,
						"locations": [{"name": "fsn1", "available": true, "recommended": false}]
					},
					{
						"id": 2,
						"name": "cx22",
						"description": "CX22",
						"cores": 2,
						"memory": 4,
						"disk": 40,
						"deprecated": false,
						"locations": [{"name": "fsn1", "available": true, "recommended": true}]
					}
				]
			}`),
		},
	}

	creator := NewCreator(client, context.Background())
	serverTypes, err := creator.ListServerTypes("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(serverTypes) != 1 {
		t.Fatalf("expected 1 non-deprecated server type, got %d", len(serverTypes))
	}

	if serverTypes[0].Name != "cx22" {
		t.Fatalf("expected server type cx22, got %s", serverTypes[0].Name)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}

	if got := client.requests[0].URL.String(); got != "https://api.hetzner.cloud/v1/server_types?per_page=50" {
		t.Fatalf("unexpected request URL: %s", got)
	}
}

func TestCreatorListAvailableLocationsHTTPError(t *testing.T) {
	client := &queueHTTPClient{
		responses: []*http.Response{
			jsonResponse(http.StatusUnauthorized, `{"error":{"message":"unauthorized"}}`),
		},
	}

	creator := NewCreator(client, context.Background())
	_, err := creator.ListAvailableLocations("bad-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "API Hetzner: HTTP 401"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestLocationListAvailable(t *testing.T) {
	location := Location{Name: "fsn1"}
	serverTypes := []ServerType{
		{
			Name: "cx22",
			Locations: []ServerTypeLocation{
				{Name: "fsn1", Available: true},
				{Name: "nbg1", Available: true},
			},
		},
		{
			Name: "cpx21",
			Locations: []ServerTypeLocation{
				{Name: "fsn1", Available: false},
				{Name: "nbg1", Available: true},
			},
		},
		{
			Name: "ccx13",
			Locations: []ServerTypeLocation{
				{Name: "nbg1", Available: true},
			},
		},
	}

	available := location.ListAvailable(serverTypes)
	if len(available) != 1 {
		t.Fatalf("expected 1 available server type, got %d", len(available))
	}

	if available[0].Name != "cx22" {
		t.Fatalf("expected cx22, got %s", available[0].Name)
	}
}

func TestServerTypeString(t *testing.T) {
	st := ServerType{
		Name:   "cx22",
		Memory: 4,
		Cores:  2,
		Disk:   40,
	}

	expected := "Name: cx22, Memory 4 GB, VCPUs: 2, Disk 40 GB"
	if got := st.String(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
