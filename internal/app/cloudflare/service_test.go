package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sword-go/internal/app/integrations"
)

func TestUpsertDNSRecordCreatesWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/zone1/dns_records":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/client/v4/zones/zone1/dns_records":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]any{"id": "rec1"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewServiceWithBase(server.Client(), integrations.Credentials{
		Type:  "api_token",
		Token: "token",
	}, server.URL+"/client/v4")

	records, err := service.UpsertDNSRecord(context.Background(), "zone1", "example.com", "A", "203.0.113.10", true, 1, "")
	if err != nil {
		t.Fatalf("upsert record: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record")
	}
}

func TestUpsertDNSRecordUpdatesExisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/zone1/dns_records":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": []any{
					map[string]any{"id": "rec1", "type": "A", "name": "example.com"},
				},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/client/v4/zones/zone1/dns_records/rec1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]any{"id": "rec1"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewServiceWithBase(server.Client(), integrations.Credentials{
		Type:  "api_token",
		Token: "token",
	}, server.URL+"/client/v4")

	records, err := service.UpsertDNSRecord(context.Background(), "zone1", "example.com", "A", "198.51.100.5", false, 1, "")
	if err != nil {
		t.Fatalf("upsert record: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record")
	}
}
