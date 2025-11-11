package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SaaSAPIClient provides helper methods for interacting with the Velocity Cache SaaS API.
type SaaSAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type PresignResponse struct {
	URL     string `json:"url"`
	Warning string `json:"warning,omitempty"`
}

// NewSaaSAPIClient constructs a new client with the provided API base URL and bearer token.
func NewSaaSAPIClient(baseURL, token string) *SaaSAPIClient {
	return &SaaSAPIClient{
		baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
	}
}

// SetHTTPClient overrides the default HTTP client used by the SaaS client.
func (c *SaaSAPIClient) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

// HTTPClient returns the HTTP client in use, instantiating a default one if necessary.
func (c *SaaSAPIClient) HTTPClient() *http.Client {
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return c.httpClient
}

// GetDownloadURL returns a presigned download URL for the provided cache key.
func (c *SaaSAPIClient) GetDownloadURL(ctx context.Context, projectID, cacheKey string) (PresignResponse, bool, error) {
	endpoint, err := c.buildURL("/api/v1/cache/download", projectID, cacheKey)
	if err != nil {
		return PresignResponse{}, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return PresignResponse{}, false, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return PresignResponse{}, false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var payload PresignResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return PresignResponse{}, false, fmt.Errorf("decode download response: %w", err)
		}
		if payload.URL == "" {
			return PresignResponse{}, false, fmt.Errorf("download response missing url")
		}
		return payload, true, nil
	case http.StatusNotFound:
		return PresignResponse{}, false, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return PresignResponse{}, false, fmt.Errorf("download request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

// GetUploadURL returns a presigned upload URL for the provided cache key.
func (c *SaaSAPIClient) GetUploadURL(ctx context.Context, projectID, cacheKey string) (PresignResponse, error) {
	endpoint, err := c.buildURL("/api/v1/cache/upload", projectID, cacheKey)
	if err != nil {
		return PresignResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return PresignResponse{}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return PresignResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return PresignResponse{}, fmt.Errorf("upload request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload PresignResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return PresignResponse{}, fmt.Errorf("decode upload response: %w", err)
	}
	if payload.URL == "" {
		return PresignResponse{}, fmt.Errorf("upload response missing url")
	}

	return payload, nil
}

func (c *SaaSAPIClient) buildURL(path, projectID, cacheKey string) (string, error) {
	base := c.baseURL
	if base == "" {
		return "", fmt.Errorf("saas api base url is empty")
	}
	full := base + path
	parsed, err := url.Parse(full)
	if err != nil {
		return "", err
	}
	q := parsed.Query()
	if cacheKey != "" {
		q.Set("key", cacheKey)
	}
	if projectID != "" {
		q.Set("projectId", projectID)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}
