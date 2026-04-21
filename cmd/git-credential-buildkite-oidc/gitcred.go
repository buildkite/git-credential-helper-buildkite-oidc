package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type credentialRequest struct {
	Protocol  string
	Authority string
	Path      string
}

type credentialResponse struct {
	Username          string
	Password          string
	PasswordExpiryUTC int64
}

func parseCredentialRequest(reader io.Reader) (credentialRequest, error) {
	scanner := bufio.NewScanner(reader)
	request := credentialRequest{}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return credentialRequest{}, fmt.Errorf("invalid credential line %q", line)
		}
		switch key {
		case "protocol":
			request.Protocol = value
		case "host":
			request.Authority = value
		case "path":
			request.Path = value
		}
	}

	if err := scanner.Err(); err != nil {
		return credentialRequest{}, err
	}

	return request, nil
}

func (r credentialResponse) Write(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "username=%s\n", r.Username); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "password=%s\n", r.Password); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "password_expiry_utc=%d\n\n", r.PasswordExpiryUTC); err != nil {
		return err
	}
	return nil
}

func normalizePathForCache(path string) string {
	normalized := strings.Trim(strings.TrimSpace(path), "/")
	if normalized == "" {
		return ""
	}
	if strings.HasSuffix(normalized, "/info/lfs") {
		normalized = strings.TrimSuffix(normalized, "/info/lfs")
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}
