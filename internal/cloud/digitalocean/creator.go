package digitalocean

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const apiBase = "https://api.digitalocean.com/v2"

var rawPublicKeyPattern = regexp.MustCompile(`^(ssh-|ecdsa-|sk-ssh-)`)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Creator struct {
	client HTTPDoer
}

func NewCreator(client HTTPDoer) *Creator {
	return &Creator{client: client}
}

func (c *Creator) Create(ctx context.Context, data CreateDropletData) (DropletResult, error) {
	if data.PublicIPPollAttempts < 1 {
		return DropletResult{}, errors.New("public IP poll attempts must be greater than zero")
	}

	if data.PublicIPPollIntervalSecs < 1 {
		return DropletResult{}, errors.New("public IP poll interval must be greater than zero")
	}

	resolvedKey, err := c.resolveSSHKey(ctx, data.PublicKey, data.APIKey)
	if err != nil {
		return DropletResult{}, err
	}

	payload := map[string]any{
		"name":     data.Name,
		"region":   data.Region,
		"size":     data.ServerType,
		"image":    data.Image,
		"ssh_keys": []any{resolvedKey.ID},
	}

	res, err := c.authorizedJSONRequest(ctx, http.MethodPost, "/droplets", data.APIKey, payload)
	if err != nil {
		return DropletResult{}, err
	}

	var body struct {
		Message string `json:"message"`
		Droplet struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			SizeSlug string `json:"size_slug"`
			Region   struct {
				Slug string `json:"slug"`
			} `json:"region"`
		} `json:"droplet"`
	}

	if err = decodeResponse(res, &body); err != nil {
		return DropletResult{}, err
	}

	if res.StatusCode != http.StatusAccepted {
		message := cmp.Or(body.Message, "Unexpected response from the DigitalOcean API.")

		return DropletResult{}, fmt.Errorf("DigitalOcean API error [%d]: %s", res.StatusCode, message)
	}

	if body.Droplet.ID == 0 || body.Droplet.Name == "" || body.Droplet.Status == "" {
		return DropletResult{}, errors.New("DigitalOcean API response did not include droplet details")
	}

	publicIP, err := c.waitForPublicIP(ctx, body.Droplet.ID, data.APIKey, data.PublicIPPollAttempts, data.PublicIPPollIntervalSecs)
	if err != nil {
		return DropletResult{}, err
	}

	region := cmp.Or(body.Droplet.Region.Slug, data.Region)
	dropletType := cmp.Or(body.Droplet.SizeSlug, data.ServerType)

	return DropletResult{
		DropletID:    body.Droplet.ID,
		Name:         body.Droplet.Name,
		Region:       region,
		Type:         dropletType,
		Status:       body.Droplet.Status,
		PublicIP:     publicIP,
		SSHKeyID:     resolvedKey.ID,
		SSHKeyName:   resolvedKey.Name,
		SSHKeyStatus: resolvedKey.Status,
	}, nil
}

type resolvedSSHKey struct {
	ID     any
	Name   *string
	Status string
}

func (c *Creator) resolveSSHKey(ctx context.Context, value string, apiKey string) (resolvedSSHKey, error) {
	if isRawPublicKey(value) {
		return c.uploadOrFindKey(ctx, value, apiKey)
	}

	id, err := c.findKeyIDByName(ctx, value, apiKey)
	if err != nil {
		return resolvedSSHKey{}, err
	}

	if id != nil {
		existingName := value
		return resolvedSSHKey{
			ID:     *id,
			Name:   &existingName,
			Status: "existing",
		}, nil
	}

	return resolvedSSHKey{
		ID:     value,
		Name:   nil,
		Status: "provided",
	}, nil
}

func isRawPublicKey(value string) bool {
	return rawPublicKeyPattern.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

func (c *Creator) uploadOrFindKey(ctx context.Context, publicKey string, apiKey string) (resolvedSSHKey, error) {
	trimmedKey := strings.TrimSpace(publicKey)
	parts := strings.Fields(trimmedKey)
	name := ""
	if len(parts) >= 3 {
		name = parts[2]
	}
	if name == "" {
		name = "key-" + randomLowerAlphaNumeric(8)
	}

	payload := map[string]any{
		"name":       name,
		"public_key": trimmedKey,
	}

	res, err := c.authorizedJSONRequest(ctx, http.MethodPost, "/account/keys", apiKey, payload)
	if err != nil {
		return resolvedSSHKey{}, err
	}

	var body struct {
		SSHKey struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"ssh_key"`
	}

	if err = decodeResponse(res, &body); err != nil {
		return resolvedSSHKey{}, err
	}

	if res.StatusCode == http.StatusCreated {
		uploadedName := body.SSHKey.Name
		return resolvedSSHKey{
			ID:     body.SSHKey.ID,
			Name:   &uploadedName,
			Status: "uploaded",
		}, nil
	}

	keys, err := c.listSSHKeys(ctx, apiKey)
	if err != nil {
		return resolvedSSHKey{}, err
	}

	for _, key := range keys {
		if strings.TrimSpace(key.PublicKey) == trimmedKey {
			existingName := key.Name
			return resolvedSSHKey{
				ID:     key.ID,
				Name:   &existingName,
				Status: "existing",
			}, nil
		}
	}

	return resolvedSSHKey{}, errors.New("could not find or upload the provided SSH key")
}

func (c *Creator) findKeyIDByName(ctx context.Context, name string, apiKey string) (*int, error) {
	keys, err := c.listSSHKeys(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if key.Name == name {
			found := key.ID
			return &found, nil
		}
	}

	return nil, nil
}

type listedSSHKey struct {
	ID        int
	Name      string
	PublicKey string
}

func (c *Creator) listSSHKeys(ctx context.Context, apiKey string) ([]listedSSHKey, error) {
	res, err := c.authorizedRequest(ctx, http.MethodGet, "/account/keys", apiKey, nil)
	if err != nil {
		return nil, err
	}

	var body struct {
		SSHKeys []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			PublicKey string `json:"public_key"`
		} `json:"ssh_keys"`
	}

	if err = decodeResponse(res, &body); err != nil {
		return nil, err
	}

	out := make([]listedSSHKey, 0, len(body.SSHKeys))
	for _, key := range body.SSHKeys {
		out = append(out, listedSSHKey{
			ID:        key.ID,
			Name:      key.Name,
			PublicKey: key.PublicKey,
		})
	}

	return out, nil
}

func (c *Creator) waitForPublicIP(ctx context.Context, dropletID int, apiKey string, attempts int, intervalSeconds int) (*string, error) {
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(intervalSeconds) * time.Second):
		}

		res, err := c.authorizedRequest(ctx, http.MethodGet, fmt.Sprintf("/droplets/%d", dropletID), apiKey, nil)
		if err != nil {
			lastErr = err
			continue
		}

		var body struct {
			Droplet struct {
				Networks struct {
					V4 []struct {
						Type      string `json:"type"`
						IPAddress string `json:"ip_address"`
					} `json:"v4"`
				} `json:"networks"`
			} `json:"droplet"`
		}

		if err = decodeResponse(res, &body); err != nil {
			lastErr = err
			continue
		}

		for _, network := range body.Droplet.Networks.V4 {
			if network.Type == "public" && network.IPAddress != "" {
				publicIP := network.IPAddress
				return &publicIP, nil
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed while waiting for the droplet public IP: %w", lastErr)
	}

	return nil, nil
}

func (c *Creator) authorizedJSONRequest(ctx context.Context, method string, path string, apiKey string, payload map[string]any) (*http.Response, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("failed to encode the DigitalOcean API request payload")
	}

	headers := map[string]string{"Content-Type": "application/json"}
	return c.authorizedRequest(ctx, method, path, apiKey, &requestBody{
		bytes:   encoded,
		headers: headers,
	})
}

type requestBody struct {
	bytes   []byte
	headers map[string]string
}

func (c *Creator) authorizedRequest(ctx context.Context, method string, path string, apiKey string, body *requestBody) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body.bytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil {
		for key, value := range body.headers {
			req.Header.Set(key, value)
		}
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	return res, nil
}

func decodeResponse(response *http.Response, out any) error {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, out); err != nil {
		return errors.New("DigitalOcean API returned an invalid JSON response")
	}

	return nil
}

const randomCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomLowerAlphaNumeric(length int) string {
	if length <= 0 {
		return ""
	}

	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		fallback := make([]byte, length)
		for index := range fallback {
			fallback[index] = randomCharset[index%len(randomCharset)]
		}
		return string(fallback)
	}

	out := make([]byte, length)
	for index, value := range randomBytes {
		out[index] = randomCharset[int(value)%len(randomCharset)]
	}

	return string(out)
}
