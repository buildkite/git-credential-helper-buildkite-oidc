package gitcred

import (
	"strings"
	"testing"
)

func TestParseRequest(t *testing.T) {
	request, err := ParseRequest(strings.NewReader("protocol=https\nhost=git.example.com\npath=acme/widgets.git\nusername=ignored\n\n"))
	if err != nil {
		t.Fatalf("ParseRequest returned error: %v", err)
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

func TestParseRequestRejectsMalformedLine(t *testing.T) {
	_, err := ParseRequest(strings.NewReader("protocol\n\n"))
	if err == nil {
		t.Fatal("expected malformed line error")
	}
}
