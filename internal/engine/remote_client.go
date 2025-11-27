package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type RemoteClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type NegotiateResponse struct {
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
}

type negotiateRequest struct {
	Hash   string `json:"hash"`
	Action string `json:"action"`
}

func NewRemoteClient(baseURL, token string) *RemoteClient {
	return &RemoteClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *RemoteClient) Negotiate(ctx context.Context, hash, action string) (*NegotiateResponse, error) {
	reqBody := negotiateRequest{
		Hash:   hash,
		Action: action,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/negotiate", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned status %d", resp.StatusCode)
	}

	var negResp NegotiateResponse
	if err := json.NewDecoder(resp.Body).Decode(&negResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &negResp, nil
}
