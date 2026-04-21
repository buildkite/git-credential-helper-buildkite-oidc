package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	httpClient  *http.Client
	exchangeURL string
}

type Request struct {
	Protocol  string `json:"protocol"`
	Authority string `json:"authority"`
	Path      string `json:"path"`
}

type Response struct {
	Password          string `json:"password"`
	PasswordExpiryUTC int64  `json:"password_expiry_utc"`
}

func New(exchangeURL string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.Parse(exchangeURL)
	if err != nil || !parsedURL.IsAbs() {
		return nil, errors.New("invalid exchange URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{httpClient: httpClient, exchangeURL: exchangeURL}, nil
}

func (c *Client) Exchange(ctx context.Context, oidcToken string, requestPayload Request) (Response, error) {
	if strings.TrimSpace(oidcToken) == "" {
		return Response{}, errors.New("missing OIDC token")
	}
	if strings.TrimSpace(requestPayload.Protocol) == "" || strings.TrimSpace(requestPayload.Authority) == "" || strings.TrimSpace(requestPayload.Path) == "" {
		return Response{}, errors.New("invalid exchange request")
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal exchange request: %w", err)
	}

	var lastErr error
	for attempt := range 3 {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.exchangeURL, bytes.NewReader(body))
		if err != nil {
			return Response{}, fmt.Errorf("create exchange request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+oidcToken)
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

		payload, retry, err := decodeExchangeResponse(response)
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

	return Response{}, lastErr
}

func decodeExchangeResponse(response *http.Response) (Response, bool, error) {
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err := fmt.Errorf("token exchange endpoint returned %s", response.Status)
		if trimmed := strings.TrimSpace(string(message)); trimmed != "" {
			err = fmt.Errorf("token exchange endpoint returned %s: %s", response.Status, trimmed)
		}
		return Response{}, retryableStatus(response.StatusCode), err
	}

	var payload Response
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Response{}, false, fmt.Errorf("decode token exchange response: %w", err)
	}
	if payload.Password == "" {
		return Response{}, false, errors.New("token exchange response missing password")
	}
	if payload.PasswordExpiryUTC == 0 {
		return Response{}, false, errors.New("token exchange response missing password_expiry_utc")
	}

	return payload, false, nil
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
