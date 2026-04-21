package main

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

const defaultBuildkiteAgentEndpoint = "https://agent-edge.buildkite.com/v3"

var fixedOIDCClaims = []string{"organization_slug", "pipeline_slug", "build_id", "job_id"}

type oidcClientConfig struct {
	BaseURL     string
	JobID       string
	AccessToken string
}

type oidcTokenRequest struct {
	Audience string   `json:"audience"`
	Claims   []string `json:"claims"`
	Lifetime int      `json:"lifetime"`
}

type oidcTokenResponse struct {
	Token string `json:"token"`
}

type exchangeRequest struct {
	Protocol  string `json:"protocol"`
	Authority string `json:"authority"`
	Path      string `json:"path"`
}

type exchangeResponse struct {
	Password          string `json:"password"`
	PasswordExpiryUTC int64  `json:"password_expiry_utc"`
}

type tokenExchangeResponse struct {
	Token      string `json:"token"`
	ValidSince int64  `json:"valid_since"`
	ExpiresAt  int64  `json:"expires_at"`
	TokenType  string `json:"token_type"`
}

func oidcClientConfigFromEnv() oidcClientConfig {
	endpoint := os.Getenv("BUILDKITE_AGENT_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultBuildkiteAgentEndpoint
	}
	return oidcClientConfig{
		BaseURL:     endpoint,
		JobID:       os.Getenv("BUILDKITE_JOB_ID"),
		AccessToken: os.Getenv("BUILDKITE_AGENT_ACCESS_TOKEN"),
	}
}

func requestOIDCToken(ctx context.Context, httpClient *http.Client, cfg oidcClientConfig, audience string, lifetimeSeconds int) (string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBuildkiteAgentEndpoint
	}
	if strings.TrimSpace(cfg.JobID) == "" {
		return "", errors.New("missing Buildkite job ID")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return "", errors.New("missing Buildkite agent access token")
	}
	if strings.TrimSpace(audience) == "" {
		return "", errors.New("missing audience")
	}
	if lifetimeSeconds <= 0 {
		return "", errors.New("invalid OIDC lifetime")
	}

	body, err := json.Marshal(oidcTokenRequest{Audience: audience, Claims: fixedOIDCClaims, Lifetime: lifetimeSeconds})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	requestURL, err := url.JoinPath(strings.TrimRight(cfg.BaseURL, "/"), "jobs", cfg.JobID, "oidc", "tokens")
	if err != nil {
		return "", fmt.Errorf("build OIDC URL: %w", err)
	}

	return doJSONPostWithRetry(ctx, httpClient, requestURL, body, map[string]string{
		"Authorization": "Token " + cfg.AccessToken,
		"Content-Type":  "application/json",
	}, decodeOIDCTokenResponse)
}

func exchangeGitCredential(ctx context.Context, httpClient *http.Client, exchangeURL, oidcToken string, requestPayload exchangeRequest) (exchangeResponse, error) {
	parsedURL, err := url.Parse(exchangeURL)
	if err != nil || !parsedURL.IsAbs() {
		return exchangeResponse{}, errors.New("invalid exchange URL")
	}
	if strings.TrimSpace(oidcToken) == "" {
		return exchangeResponse{}, errors.New("missing OIDC token")
	}
	if strings.TrimSpace(requestPayload.Protocol) == "" || strings.TrimSpace(requestPayload.Authority) == "" || strings.TrimSpace(requestPayload.Path) == "" {
		return exchangeResponse{}, errors.New("invalid exchange request")
	}

	return doJSONPostWithRetry(ctx, httpClient, exchangeURL, nil, map[string]string{
		"Authorization": "Bearer " + oidcToken,
	}, func(response *http.Response) (exchangeResponse, bool, error) {
		return decodeExchangeResponse(response, requestPayload)
	})
}

func decodeOIDCTokenResponse(response *http.Response) (string, bool, error) {
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

	var payload oidcTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", false, fmt.Errorf("decode Buildkite OIDC response: %w", err)
	}
	if payload.Token == "" {
		return "", false, errors.New("buildkite OIDC response missing token")
	}

	return payload.Token, false, nil
}

func decodeExchangeResponse(response *http.Response, requestPayload exchangeRequest) (exchangeResponse, bool, error) {
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err := fmt.Errorf("token exchange endpoint returned %s", response.Status)
		if trimmed := strings.TrimSpace(string(message)); trimmed != "" {
			err = fmt.Errorf("token exchange endpoint returned %s: %s", response.Status, trimmed)
		}
		return exchangeResponse{}, retryableStatus(response.StatusCode), err
	}

	var payload tokenExchangeResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return exchangeResponse{}, false, fmt.Errorf("decode token exchange response: %w", err)
	}
	if payload.Token == "" {
		return exchangeResponse{}, false, errors.New("token exchange response missing token")
	}
	if payload.ExpiresAt == 0 {
		return exchangeResponse{}, false, errors.New("token exchange response missing expires_at")
	}

	return exchangeResponse{Password: payload.Token, PasswordExpiryUTC: payload.ExpiresAt}, false, nil
}

func doJSONPostWithRetry[T any](ctx context.Context, httpClient *http.Client, requestURL string, body []byte, headers map[string]string, decode func(*http.Response) (T, bool, error)) (T, error) {
	var zero T
	httpClient = defaultHTTPClient(httpClient)

	var lastErr error
	for attempt := range 3 {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
		if err != nil {
			return zero, fmt.Errorf("create request: %w", err)
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}

		response, err := httpClient.Do(request)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				sleepBackoff(ctx, attempt)
				continue
			}
			break
		}

		payload, retry, err := decode(response)
		if err == nil {
			return payload, nil
		}
		lastErr = err
		if retry && attempt < 2 {
			sleepBackoff(ctx, attempt)
			continue
		}
		break
	}

	return zero, lastErr
}

func defaultHTTPClient(httpClient *http.Client) *http.Client {
	if httpClient != nil {
		return httpClient
	}
	return &http.Client{Timeout: 10 * time.Second}
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
