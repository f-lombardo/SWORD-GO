package cloud

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sword-go/internal/utils"
)

var rawPublicKeyPattern = regexp.MustCompile(`^(ssh-|ecdsa-|sk-ssh-)`)

// IsRawPublicKey reports whether a string looks like a raw SSH public key.
func IsRawPublicKey(value string) bool {
	return rawPublicKeyPattern.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

// ResolvedSSHKey represents a resolved key on a cloud provider.
type ResolvedSSHKey struct {
	ID     any
	Name   *string
	Status string
}

// ListedSSHKey represents an SSH key entry returned by provider listings.
type ListedSSHKey struct {
	ID        int
	Name      string
	PublicKey string
}

// SSHKeyService handles SSH key resolution and management across cloud providers.
type SSHKeyService struct {
	client   *Client
	endpoint string
}

// NewSSHKeyService initializes an SSH key service for a specific provider endpoint.
func NewSSHKeyService(client *Client, endpoint string) *SSHKeyService {
	return &SSHKeyService{
		client:   client,
		endpoint: endpoint,
	}
}

// Resolve resolves an SSH key by raw public key content, existing name, or passthrough ID.
func (s *SSHKeyService) Resolve(ctx context.Context, value string, apiKey string) (ResolvedSSHKey, error) {
	if IsRawPublicKey(value) {
		return s.UploadOrFindKey(ctx, value, apiKey)
	}

	id, err := s.FindKeyIDByName(ctx, value, apiKey)
	if err != nil {
		return ResolvedSSHKey{}, err
	}

	if id != nil {
		existingName := value
		return ResolvedSSHKey{
			ID:     *id,
			Name:   &existingName,
			Status: "existing",
		}, nil
	}

	return ResolvedSSHKey{
		ID:     value,
		Name:   nil,
		Status: "provided",
	}, nil
}

// UploadOrFindKey uploads a public key or looks it up among existing keys if already present.
func (s *SSHKeyService) UploadOrFindKey(ctx context.Context, publicKey string, apiKey string) (ResolvedSSHKey, error) {
	trimmedKey := strings.TrimSpace(publicKey)
	parts := strings.Fields(trimmedKey)
	name := ""
	if len(parts) >= 3 {
		name = parts[2]
	}
	if name == "" {
		name = "key-" + utils.RandomLowerAlphaNumeric(8)
	}

	payload := map[string]any{
		"name":       name,
		"public_key": trimmedKey,
	}

	res, err := s.client.JSONRequest(ctx, http.MethodPost, s.endpoint, apiKey, payload)
	if err != nil {
		return ResolvedSSHKey{}, err
	}

	var body struct {
		SSHKey struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"ssh_key"`
	}

	if err = DecodeResponse(res, &body); err != nil {
		return ResolvedSSHKey{}, err
	}

	if res.StatusCode == http.StatusCreated {
		uploadedName := body.SSHKey.Name
		return ResolvedSSHKey{
			ID:     body.SSHKey.ID,
			Name:   &uploadedName,
			Status: "uploaded",
		}, nil
	}

	keys, err := s.ListSSHKeys(ctx, apiKey)
	if err != nil {
		return ResolvedSSHKey{}, err
	}

	for _, key := range keys {
		if strings.TrimSpace(key.PublicKey) == trimmedKey {
			existingName := key.Name
			return ResolvedSSHKey{
				ID:     key.ID,
				Name:   &existingName,
				Status: "existing",
			}, nil
		}
	}

	return ResolvedSSHKey{}, errors.New("could not find or upload the provided SSH key")
}

// FindKeyIDByName locates an existing key by name.
func (s *SSHKeyService) FindKeyIDByName(ctx context.Context, name string, apiKey string) (*int, error) {
	keys, err := s.ListSSHKeys(ctx, apiKey)
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

// ListSSHKeys lists all SSH keys configured on the cloud provider account.
func (s *SSHKeyService) ListSSHKeys(ctx context.Context, apiKey string) ([]ListedSSHKey, error) {
	res, err := s.client.Request(ctx, http.MethodGet, s.endpoint, apiKey, nil, nil)
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

	if err = DecodeResponse(res, &body); err != nil {
		return nil, err
	}

	out := make([]ListedSSHKey, 0, len(body.SSHKeys))
	for _, key := range body.SSHKeys {
		out = append(out, ListedSSHKey{
			ID:        key.ID,
			Name:      key.Name,
			PublicKey: key.PublicKey,
		})
	}

	return out, nil
}
