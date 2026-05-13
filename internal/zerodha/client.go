package zerodha

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.kite.trade"

// Client is a stateless HTTP client for the Kite Connect v3 API.
// Each call requires the caller to pass an access_token; the client
// holds only the api_key (public) and an http.Client.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient creates a Kite Connect client for the given api_key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// LoginURL returns the Kite web login URL with a redirect_params state value.
// redirectURL is the callback endpoint (e.g. http://localhost:8082/api/zerodha/callback).
func (c *Client) LoginURL(redirectURL, state string) string {
	redirectParams := url.QueryEscape("state=" + state)
	return fmt.Sprintf(
		"https://kite.zerodha.com/connect/login?v=3&api_key=%s&redirect_params=%s",
		c.apiKey, redirectParams,
	)
}

// ExchangeToken exchanges a request_token for an access_token.
// checksum = SHA-256(api_key + request_token + api_secret).
func (c *Client) ExchangeToken(ctx context.Context, requestToken, apiSecret string) (*TokenResponse, error) {
	// lgtm[go/weak-sensitive-data-hashing] -- Kite Connect v3 mandates SHA-256(api_key+request_token+api_secret) as the session checksum; not password storage
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(c.apiKey+requestToken+apiSecret)))

	form := url.Values{
		"api_key":       {c.apiKey},
		"request_token": {requestToken},
		"checksum":      {checksum},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/session/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Kite-Version", "3")

	var resp apiResponse[TokenResponse]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetHoldings fetches equity + SGB holdings from /portfolio/holdings.
func (c *Client) GetHoldings(ctx context.Context, accessToken string) ([]Holding, error) {
	req, err := c.newGet(ctx, "/portfolio/holdings", accessToken)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Holding]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetMFHoldings fetches mutual fund holdings from /mf/holdings.
func (c *Client) GetMFHoldings(ctx context.Context, accessToken string) ([]MFHolding, error) {
	req, err := c.newGet(ctx, "/mf/holdings", accessToken)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]MFHolding]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) newGet(ctx context.Context, path, accessToken string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.apiKey+":"+accessToken)
	req.Header.Set("X-Kite-Version", "3")
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kite request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp apiResponse[any]
		_ = json.Unmarshal(body, &errResp)
		return fmt.Errorf("kite %s: %s", resp.Status, errResp.Message)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
