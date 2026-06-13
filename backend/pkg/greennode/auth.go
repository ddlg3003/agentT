package greennode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultOAuth2TokenURL is the GreenNode IAM token endpoint. Override via the
// GREENNODE_OAUTH2_TOKEN_URL env var / config key.
const DefaultOAuth2TokenURL = "https://iam.api.vngcloud.vn/accounts-api/v2/auth/token"

// defaultTokenCacheTTL is used when the token response omits expires_in.
const defaultTokenCacheTTL = 55 * time.Minute

// TokenSource fetches and caches OAuth2 access tokens using the
// client_credentials grant, mirroring the Python SDK's AuthenticatedAPIClient.
// It is safe for concurrent use.
type TokenSource struct {
	creds      IAMCredentials
	tokenURL   string
	httpClient *http.Client

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

// NewTokenSource builds a TokenSource. If tokenURL is empty, it resolves from
// config (GREENNODE_OAUTH2_TOKEN_URL) falling back to DefaultOAuth2TokenURL.
func NewTokenSource(creds IAMCredentials, tokenURL string, httpClient *http.Client) *TokenSource {
	if tokenURL == "" {
		tokenURL = GetConfigValue("GREENNODE_OAUTH2_TOKEN_URL", DefaultOAuth2TokenURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &TokenSource{creds: creds, tokenURL: tokenURL, httpClient: httpClient}
}

func (ts *TokenSource) valid() bool {
	// 60s safety buffer so we never hand out an about-to-expire token.
	return ts.cached != "" && time.Now().Before(ts.expiresAt.Add(-60*time.Second))
}

// Token returns a valid access token, fetching a new one when the cache is
// empty or expired.
func (ts *TokenSource) Token(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.valid() {
		return ts.cached, nil
	}
	if err := ts.creds.Require(); err != nil {
		return "", err
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", RequestError("failed to build token request", 0, err)
	}
	basic := base64.StdEncoding.EncodeToString([]byte(ts.creds.ClientID + ":" + ts.creds.ClientSecret))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.httpClient.Do(req)
	if err != nil {
		return "", RequestError("OAuth2 token request failed", 0, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", RequestError("OAuth2 token request failed", resp.StatusCode, nil)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", RequestError("failed to decode token response", resp.StatusCode, err)
	}
	if body.AccessToken == "" {
		return "", RequestError("token response missing access_token", resp.StatusCode, nil)
	}

	ttl := defaultTokenCacheTTL
	if body.ExpiresIn > 0 {
		ttl = time.Duration(float64(body.ExpiresIn)*0.9) * time.Second
	}
	ts.cached = body.AccessToken
	ts.expiresAt = time.Now().Add(ttl)
	return ts.cached, nil
}
