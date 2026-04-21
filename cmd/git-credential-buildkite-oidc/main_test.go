package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunGetUsesExchangeAndCache(t *testing.T) {
	var oidcCalls atomic.Int32
	var exchangeCalls atomic.Int32

	oidcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oidcCalls.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected OIDC method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Token agent-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		if r.URL.Path != "/v3/jobs/job-123/oidc/tokens" {
			t.Fatalf("unexpected OIDC path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"oidc-token"}`)
	}))
	t.Cleanup(oidcServer.Close)

	exchangeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchangeCalls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer oidc-token" {
			t.Fatalf("unexpected exchange auth header: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read exchange body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("unexpected exchange body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"git-password","expires_in":270,"expires_at":1893456000,"token_type":"bearer","allowed_repos":["acme/widgets"]}`)
	}))
	t.Cleanup(exchangeServer.Close)

	t.Setenv("BUILDKITE_AGENT_ENDPOINT", oidcServer.URL+"/v3")
	t.Setenv("BUILDKITE_AGENT_ACCESS_TOKEN", "agent-token")
	t.Setenv("BUILDKITE_JOB_ID", "job-123")

	args := []string{
		"--exchange-url=" + exchangeServer.URL,
		"--audience=git-token-exchange",
		"--allowed-authority=git.example.com",
		"--cache-dir=" + filepath.Join(t.TempDir(), "cache"),
		"get",
	}
	stdin := "protocol=https\nhost=git.example.com\npath=acme/widgets.git\n\n"

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	if exitCode := run(args, strings.NewReader(stdin), stdout, stderr); exitCode != 0 {
		t.Fatalf("first run failed with %d: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, fragment := range []string{"username=buildkite-agent", "password=git-password", "password_expiry_utc=1893456000"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("stdout missing %q: %s", fragment, output)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := run(args, strings.NewReader(stdin), stdout, stderr); exitCode != 0 {
		t.Fatalf("second run failed with %d: %s", exitCode, stderr.String())
	}

	if oidcCalls.Load() != 1 {
		t.Fatalf("expected one OIDC call, got %d", oidcCalls.Load())
	}
	if exchangeCalls.Load() != 1 {
		t.Fatalf("expected one exchange call, got %d", exchangeCalls.Load())
	}
}

func TestRunGetRejectsMissingPath(t *testing.T) {
	t.Setenv("BUILDKITE_JOB_ID", "job-123")

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	args := []string{
		"--exchange-url=https://auth.example.com/api/git-credentials/exchange",
		"--audience=git-token-exchange",
		"--allowed-authority=git.example.com",
		"get",
	}
	stdin := "protocol=https\nhost=git.example.com\n\n"

	if exitCode := run(args, strings.NewReader(stdin), stdout, stderr); exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "missing path") {
		t.Fatalf("expected missing path error, got %s", stderr.String())
	}
}
