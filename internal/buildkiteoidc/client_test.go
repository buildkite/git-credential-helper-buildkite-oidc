package buildkiteoidc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRequestToken(t *testing.T) {
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

	client, err := New(server.URL+"/v3", "job-123", "agent-token", server.Client())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	token, err := client.RequestToken(t.Context(), "git-token-exchange", 300)
	if err != nil {
		t.Fatalf("RequestToken returned error: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestRequestTokenRetriesTransientFailures(t *testing.T) {
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

	client, err := New(server.URL, "job-123", "agent-token", server.Client())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	token, err := client.RequestToken(t.Context(), "git-token-exchange", 300)
	if err != nil {
		t.Fatalf("RequestToken returned error: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("unexpected token: %s", token)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}
