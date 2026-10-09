package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"sword-go/internal/app/integrations"
)

const defaultAPIBase = "https://api.cloudflare.com/client/v4"

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Service struct {
	client      HTTPDoer
	apiBase     string
	credentials integrations.Credentials
}

func NewService(client HTTPDoer, credentials integrations.Credentials) *Service {
	return &Service{
		client:      client,
		apiBase:     defaultAPIBase,
		credentials: credentials,
	}
}

func NewServiceWithBase(client HTTPDoer, credentials integrations.Credentials, apiBase string) *Service {
	return &Service{
		client:      client,
		apiBase:     strings.TrimRight(apiBase, "/"),
		credentials: credentials,
	}
}

func (s *Service) GetZones(ctx context.Context) ([]map[string]any, error) {
	response, err := s.send(ctx, http.MethodGet, "/zones?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	return extractResultList(payload), nil
}

func (s *Service) GetZone(ctx context.Context, zoneID string) (map[string]any, error) {
	response, err := s.send(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID), nil)
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	if result, ok := payload["result"].(map[string]any); ok {
		return result, nil
	}
	return map[string]any{}, nil
}

func (s *Service) GetDNSRecords(ctx context.Context, zoneID string) ([]map[string]any, error) {
	response, err := s.send(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/dns_records?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	return extractResultList(payload), nil
}

func (s *Service) GetZoneAnalytics(ctx context.Context, zoneID string) (map[string]any, error) {
	response, err := s.send(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/analytics/dashboard", nil)
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	if result, ok := payload["result"].(map[string]any); ok {
		return result, nil
	}
	return map[string]any{}, nil
}

func (s *Service) GetSSLSettings(ctx context.Context, zoneID string) (map[string]any, error) {
	response, err := s.send(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/settings/ssl", nil)
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	if result, ok := payload["result"].(map[string]any); ok {
		return result, nil
	}
	return map[string]any{}, nil
}

func (s *Service) PurgeCache(ctx context.Context, zoneID string) (bool, error) {
	response, err := s.send(ctx, http.MethodPost, "/zones/"+url.PathEscape(zoneID)+"/purge_cache", map[string]any{
		"purge_everything": true,
	})
	if err != nil {
		return false, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return false, err
	}
	success, _ := payload["success"].(bool)
	return success, nil
}

func (s *Service) CreateDNSRecord(ctx context.Context, zoneID string, recordType string, name string, content string, proxied bool, ttl int) (map[string]any, error) {
	response, err := s.send(ctx, http.MethodPost, "/zones/"+url.PathEscape(zoneID)+"/dns_records", map[string]any{
		"type":    strings.ToUpper(recordType),
		"name":    name,
		"content": content,
		"proxied": proxied,
		"ttl":     ttl,
	})
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	if result, ok := payload["result"].(map[string]any); ok {
		return result, nil
	}
	return map[string]any{}, nil
}

func (s *Service) UpdateDNSRecord(ctx context.Context, zoneID string, recordID string, recordType string, name string, content string, proxied bool, ttl int) (map[string]any, error) {
	response, err := s.send(ctx, http.MethodPatch, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), map[string]any{
		"type":    strings.ToUpper(recordType),
		"name":    name,
		"content": content,
		"proxied": proxied,
		"ttl":     ttl,
	})
	if err != nil {
		return nil, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return nil, err
	}
	if result, ok := payload["result"].(map[string]any); ok {
		return result, nil
	}
	return map[string]any{}, nil
}

func (s *Service) DeleteDNSRecord(ctx context.Context, zoneID string, recordID string) (bool, error) {
	response, err := s.send(ctx, http.MethodDelete, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil)
	if err != nil {
		return false, err
	}
	payload, err := decodePayload(response)
	if err != nil {
		return false, err
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		return false, nil
	}
	_, found := result["id"]
	return found, nil
}

func (s *Service) FindZoneForDomain(ctx context.Context, domain string) (map[string]any, bool, error) {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	parts := strings.Split(domain, ".")
	candidates := make([]string, 0)
	for index := len(parts) - 2; index >= 0; index-- {
		candidates = append(candidates, strings.Join(parts[index:], "."))
	}

	zones, err := s.GetZones(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, candidate := range candidates {
		for _, zone := range zones {
			name, _ := zone["name"].(string)
			if name == candidate {
				return zone, true, nil
			}
		}
	}
	return nil, false, nil
}

func (s *Service) UpsertDNSRecord(ctx context.Context, zoneID string, name string, recordType string, content string, proxied bool, ttl int, cnameContent string) ([]map[string]any, error) {
	if recordType == "both" {
		results := make([]map[string]any, 0, 2)
		first, err := s.upsertSingle(ctx, zoneID, "A", name, content, proxied, ttl)
		if err != nil {
			return nil, err
		}
		results = append(results, first)
		if strings.TrimSpace(cnameContent) != "" {
			second, err := s.upsertSingle(ctx, zoneID, "CNAME", name, cnameContent, proxied, ttl)
			if err != nil {
				return nil, err
			}
			results = append(results, second)
		}
		return results, nil
	}

	record, err := s.upsertSingle(ctx, zoneID, recordType, name, content, proxied, ttl)
	if err != nil {
		return nil, err
	}
	return []map[string]any{record}, nil
}

func (s *Service) upsertSingle(ctx context.Context, zoneID string, recordType string, name string, content string, proxied bool, ttl int) (map[string]any, error) {
	records, err := s.GetDNSRecords(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	normalizedType := strings.ToUpper(strings.TrimSpace(recordType))
	for _, record := range records {
		recordName, _ := record["name"].(string)
		recordTypeValue, _ := record["type"].(string)
		if strings.ToLower(recordName) == normalizedName && strings.ToUpper(recordTypeValue) == normalizedType {
			recordID, _ := record["id"].(string)
			return s.UpdateDNSRecord(ctx, zoneID, recordID, normalizedType, name, content, proxied, ttl)
		}
	}
	return s.CreateDNSRecord(ctx, zoneID, normalizedType, name, content, proxied, ttl)
}

func (s *Service) send(ctx context.Context, method string, path string, body map[string]any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(encoded)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.apiBase+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.credentials.Type == "api_token" {
		req.Header.Set("Authorization", "Bearer "+s.credentials.Token)
	} else {
		req.Header.Set("X-Auth-Email", s.credentials.Email)
		req.Header.Set("X-Auth-Key", s.credentials.Key)
	}

	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func decodePayload(response *http.Response) (map[string]any, error) {
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	success, _ := payload["success"].(bool)
	if response.StatusCode >= 400 || (!success && response.StatusCode >= 200 && response.StatusCode < 300) {
		if errorsList, ok := payload["errors"].([]any); ok && len(errorsList) > 0 {
			if first, ok := errorsList[0].(map[string]any); ok {
				if message, ok := first["message"].(string); ok {
					return nil, errors.New(message)
				}
			}
		}
	}
	return payload, nil
}

func extractResultList(payload map[string]any) []map[string]any {
	raw, ok := payload["result"].([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if mapped, ok := item.(map[string]any); ok {
			out = append(out, mapped)
		}
	}
	return out
}
