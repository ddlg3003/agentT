package greennode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RESTClient is a thin JSON-over-HTTP client that injects an OAuth2 bearer
// token (from a TokenSource) on every request. Service clients (e.g. the Memory
// client) compose one of these.
type RESTClient struct {
	BaseURL     string
	HTTPClient  *http.Client
	TokenSource *TokenSource
}

// NewRESTClient creates a RESTClient. A default 30s-timeout http.Client is used
// when httpClient is nil.
func NewRESTClient(baseURL string, ts *TokenSource, httpClient *http.Client) *RESTClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &RESTClient{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		HTTPClient:  httpClient,
		TokenSource: ts,
	}
}

// Do performs an authenticated request. body, when non-nil, is JSON-encoded;
// out, when non-nil, receives the decoded JSON response. query may be nil.
func (c *RESTClient) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return RequestError("failed to encode request body", 0, err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return RequestError("failed to build request", 0, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	if c.TokenSource != nil {
		token, err := c.TokenSource.Token(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return RequestError(fmt.Sprintf("%s %s failed", method, path), 0, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		// Surface a server-provided "message" field when present.
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		return RequestError(fmt.Sprintf("%s %s: %s", method, path, msg), resp.StatusCode, nil)
	}

	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return RequestError(fmt.Sprintf("failed to decode response (status %d): %s", resp.StatusCode, string(data)), resp.StatusCode, err)
	}
	return nil
}
