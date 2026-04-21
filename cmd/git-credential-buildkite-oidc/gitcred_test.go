package main

import (
	"strings"
	"testing"
)

func TestParseCredentialRequest(t *testing.T) {
	request, err := parseCredentialRequest(strings.NewReader("protocol=https\nhost=git.example.com\npath=acme/widgets.git\nusername=ignored\n\n"))
	if err != nil {
		t.Fatalf("parseCredentialRequest returned error: %v", err)
	}
	if request.Protocol != "https" {
		t.Fatalf("unexpected protocol: %s", request.Protocol)
	}
	if request.Authority != "git.example.com" {
		t.Fatalf("unexpected authority: %s", request.Authority)
	}
	if request.Path != "acme/widgets.git" {
		t.Fatalf("unexpected path: %s", request.Path)
	}
}

func TestParseCredentialRequestRejectsMalformedLine(t *testing.T) {
	_, err := parseCredentialRequest(strings.NewReader("protocol\n\n"))
	if err == nil {
		t.Fatal("expected malformed line error")
	}
}

func TestNormalizePathForCache(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "trim slashes", input: "/acme/widgets.git/", expected: "acme/widgets.git"},
		{name: "lfs path", input: "/acme/widgets.git/info/lfs", expected: "acme/widgets.git"},
		{name: "empty path", input: "/", expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := normalizePathForCache(testCase.input); actual != testCase.expected {
				t.Fatalf("normalizePathForCache(%q) = %q, want %q", testCase.input, actual, testCase.expected)
			}
		})
	}
}

func TestNormalizePathForAuthorization(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "trim git suffix", input: "/acme/widgets.git", expected: "acme/widgets"},
		{name: "lfs path", input: "/acme/widgets.git/info/lfs", expected: "acme/widgets"},
		{name: "plain path", input: "acme/widgets", expected: "acme/widgets"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := normalizePathForAuthorization(testCase.input); actual != testCase.expected {
				t.Fatalf("normalizePathForAuthorization(%q) = %q, want %q", testCase.input, actual, testCase.expected)
			}
		})
	}
}
