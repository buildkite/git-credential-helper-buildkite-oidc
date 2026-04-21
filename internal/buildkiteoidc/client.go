package buildkiteoidc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultEndpoint = "https://agent-edge.buildkite.com/v3"

var fixedClaims = []string{"organization_slug", "pipeline_slug", "build_id", "job_id"}

type Client struct {
	httpClient  *http.Client
	baseURL     string
	jobID       string
	accessToken string
}

type tokenRequest struct {
	Audience string   `json:"audience"`
	Claims   []string `json:"claims"`
	Lifetime int      `json:"lifetime"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

func New(baseURL, jobID, accessToken string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultEndpoint
	}
	if strings.TrimSpace(jobID) == "" {
		return nil, errors.New("missing Buildkite job ID")
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("missing Buildkite agent access token")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/"), jobID: jobID, accessToken: accessToken}, nil
}

func NewFromEnv(httpClient *http.Client) (*Client, error) {
	endpoint := os.Getenv("BUILDKITE_AGENT_ENDPOINT")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return New(endpoint, os.Getenv("BUILDKITE_JOB_ID"), os.Getenv("BUILDKITE_AGENT_ACCESS_TOKEN"), httpClient)
}

func (c *Client) RequestToken(ctx context.Context, audience string, lifetimeSeconds int) (string, error) {
	if strings.TrimSpace(audience) == "" {
		return "", errors.New("missing audience")
	}
	if lifetimeSeconds <= 0 {
		return "", errors.New("invalid OIDC lifetime")
	}

	body, err := json.Marshal(tokenRequest{Audience: audience, Claims: fixedClaims, Lifetime: lifetimeSeconds})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	requestURL, err := url.JoinPath(c.baseURL, "jobs", c.jobID, "oidc", "tokens")
	if err != nil {
		return "", fmt.Errorf("build OIDC URL: %w", err)
	}

	var lastErr error
	for attempt := range 3 {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		request.Header.Set("Authorization", "Token "+c.accessToken)
		request.Header.Set("Content-Type", "application/json")

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				sleepBackoff(ctx, attempt)
				continue
			}
			break
		}

		token, retry, err := decodeTokenResponse(response)
		if err == nil {
			return token, nil
		}
		lastErr = err
		if retry && attempt < 2 {
			sleepBackoff(ctx, attempt)
			continue
		}
		break
	}

	return "", lastErr
}

func decodeTokenResponse(response *http.Response) (string, bool, error) {
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err := fmt.Errorf("buildkite OIDC endpoint returned %s", response.Status)
		if trimmed := strings.TrimSpace(string(message)); trimmed != "" {
			err = fmt.Errorf("buildkite OIDC endpoint returned %s: %s", response.Status, trimmed)
		}
		return "", retryableStatus(response.StatusCode), err
	}

	var payload tokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", false, fmt.Errorf("decode Buildkite OIDC response: %w", err)
	}
	if payload.Token == "" {
		return "", false, errors.New("buildkite OIDC response missing token")
	}

	return payload.Token, false, nil
}

func retryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func sleepBackoff(ctx context.Context, attempt int) {
	backoff := time.Duration(attempt+1) * 200 * time.Millisecond
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
