package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Transfer executes a file transfer (upload or download).
// For Upload (PUT): body is the file content.
// For Download (GET): output is where the response body will be written.
// serverURL is used to determine if the Authorization header should be sent.
func Transfer(ctx context.Context, method, targetURL, serverURL string, body io.Reader, output io.Writer, contentLength int64, authToken string) error {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.ContentLength = contentLength
	}

	// Security Check: Only send auth token if target matches server
	shouldAddAuth, err := hostsMatch(targetURL, serverURL)
	if err != nil {
		return fmt.Errorf("check host match: %w", err)
	}

	if shouldAddAuth && authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transfer failed with status %d", resp.StatusCode)
	}

	if output != nil {
		if _, err := io.Copy(output, resp.Body); err != nil {
			return fmt.Errorf("copy response body: %w", err)
		}
	}

	return nil
}

func hostsMatch(url1, url2 string) (bool, error) {
	u1, err := url.Parse(url1)
	if err != nil {
		return false, err
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(u1.Host, u2.Host), nil
}
