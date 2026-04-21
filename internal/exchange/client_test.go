package exchange

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oidc-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), `"authority":"git.example.com"`) {
			t.Fatalf("unexpected body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"password":"git-password","password_expiry_utc":1893456000}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	response, err := client.Exchange(t.Context(), "oidc-token", Request{
		Protocol:  "https",
		Authority: "git.example.com",
		Path:      "acme/widgets.git",
	})
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if response.Password != "git-password" {
		t.Fatalf("unexpected password: %s", response.Password)
	}
	if response.PasswordExpiryUTC != 1893456000 {
		t.Fatalf("unexpected expiry: %d", response.PasswordExpiryUTC)
	}
}

func TestExchangeRejectsMissingExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"password":"git-password"}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = client.Exchange(t.Context(), "oidc-token", Request{
		Protocol:  "https",
		Authority: "git.example.com",
		Path:      "acme/widgets.git",
	})
	if err == nil || !strings.Contains(err.Error(), "password_expiry_utc") {
		t.Fatalf("expected missing expiry error, got %v", err)
	}
}
