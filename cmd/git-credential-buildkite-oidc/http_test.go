package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRequestOIDCToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/jobs/job-123/oidc/tokens" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token agent-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), `"claims":["organization_slug","pipeline_slug","build_id","job_id"]`) {
			t.Fatalf("unexpected claims body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"jwt-token"}`)
	}))
	t.Cleanup(server.Close)

	token, err := requestOIDCToken(t.Context(), server.Client(), oidcClientConfig{
		BaseURL:     server.URL + "/v3",
		JobID:       "job-123",
		AccessToken: "agent-token",
	}, "git-token-exchange", 300)
	if err != nil {
		t.Fatalf("requestOIDCToken returned error: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestRequestOIDCTokenRetriesTransientFailures(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"jwt-token"}`)
	}))
	t.Cleanup(server.Close)

	token, err := requestOIDCToken(t.Context(), server.Client(), oidcClientConfig{
		BaseURL:     server.URL,
		JobID:       "job-123",
		AccessToken: "agent-token",
	}, "git-token-exchange", 300)
	if err != nil {
		t.Fatalf("requestOIDCToken returned error: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("unexpected token: %s", token)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestExchangeGitCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oidc-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("unexpected body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"git-password","expires_in":270,"expires_at":1893456000,"token_type":"bearer","allowed_repos":["acme/widgets"]}`)
	}))
	t.Cleanup(server.Close)

	response, err := exchangeGitCredential(t.Context(), server.Client(), server.URL, "oidc-token", exchangeRequest{
		Protocol:  "https",
		Authority: "git.example.com",
		Path:      "acme/widgets.git",
	})
	if err != nil {
		t.Fatalf("exchangeGitCredential returned error: %v", err)
	}
	if response.Password != "git-password" {
		t.Fatalf("unexpected password: %s", response.Password)
	}
	if response.PasswordExpiryUTC != 1893456000 {
		t.Fatalf("unexpected expiry: %d", response.PasswordExpiryUTC)
	}
}

func TestExchangeGitCredentialRejectsMissingExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"git-password","allowed_repos":["acme/widgets"]}`)
	}))
	t.Cleanup(server.Close)

	_, err := exchangeGitCredential(t.Context(), server.Client(), server.URL, "oidc-token", exchangeRequest{
		Protocol:  "https",
		Authority: "git.example.com",
		Path:      "acme/widgets.git",
	})
	if err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("expected missing expiry error, got %v", err)
	}
}

func TestExchangeGitCredentialRejectsRepoOutsideAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"git-password","expires_at":1893456000,"token_type":"bearer","allowed_repos":["acme/other-repo"]}`)
	}))
	t.Cleanup(server.Close)

	_, err := exchangeGitCredential(t.Context(), server.Client(), server.URL, "oidc-token", exchangeRequest{
		Protocol:  "https",
		Authority: "git.example.com",
		Path:      "acme/widgets.git",
	})
	if err == nil || !strings.Contains(err.Error(), `does not allow repo "acme/widgets"`) {
		t.Fatalf("expected repo allowlist error, got %v", err)
	}
}
