package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/buildkite/git-credential-helper-buildkite-oidc/internal/buildkiteoidc"
	"github.com/buildkite/git-credential-helper-buildkite-oidc/internal/cache"
	"github.com/buildkite/git-credential-helper-buildkite-oidc/internal/exchange"
	"github.com/buildkite/git-credential-helper-buildkite-oidc/internal/gitcred"
	"github.com/buildkite/git-credential-helper-buildkite-oidc/internal/repoid"
)

const cacheRefreshSkew = 45 * time.Second

var version = "dev"

type config struct {
	exchangeURL      string
	audience         string
	allowedAuthority string
	username         string
	cacheBaseDir     string
	oidcLifetime     int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, operation, err := parseArgs(args, stderr)
	if err != nil {
		writeStderr(stderr, "%v\n", err)
		return 2
	}

	switch operation {
	case "version":
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	case "store":
		return 0
	case "get":
		return runGet(cfg, stdin, stdout, stderr)
	case "erase":
		return runErase(cfg, stdin, stderr)
	default:
		writeStderr(stderr, "unsupported operation %q\n", operation)
		return 2
	}
}

func parseArgs(args []string, stderr io.Writer) (config, string, error) {
	fs := flag.NewFlagSet("git-credential-buildkite-oidc", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cfg := config{}
	var showVersion bool
	fs.StringVar(&cfg.exchangeURL, "exchange-url", "", "token exchange URL")
	fs.StringVar(&cfg.audience, "audience", "", "Buildkite OIDC audience")
	fs.StringVar(&cfg.allowedAuthority, "allowed-authority", "", "allowed Git authority")
	fs.StringVar(&cfg.username, "username", "buildkite-agent", "username returned to Git")
	fs.StringVar(&cfg.cacheBaseDir, "cache-dir", cache.DefaultBaseDir(), "credential cache base directory")
	fs.IntVar(&cfg.oidcLifetime, "oidc-lifetime", 300, "Buildkite OIDC lifetime in seconds")
	fs.BoolVar(&showVersion, "version", false, "print version")

	if err := fs.Parse(args); err != nil {
		return config{}, "", err
	}

	remaining := fs.Args()
	if showVersion {
		if len(remaining) != 0 {
			return config{}, "", errors.New("--version does not accept an operation")
		}
		return cfg, "version", nil
	}
	if len(remaining) != 1 {
		return config{}, "", errors.New("expected exactly one operation: get, store, or erase")
	}

	operation := remaining[0]
	if operation == "store" {
		return cfg, operation, nil
	}

	if cfg.exchangeURL == "" {
		return config{}, "", errors.New("--exchange-url is required")
	}
	parsedExchangeURL, err := url.Parse(cfg.exchangeURL)
	if err != nil || !parsedExchangeURL.IsAbs() {
		return config{}, "", errors.New("--exchange-url must be an absolute URL")
	}
	if strings.TrimSpace(cfg.audience) == "" {
		return config{}, "", errors.New("--audience is required")
	}
	if strings.TrimSpace(cfg.allowedAuthority) == "" {
		return config{}, "", errors.New("--allowed-authority is required")
	}
	if strings.Contains(cfg.allowedAuthority, "://") {
		return config{}, "", errors.New("--allowed-authority must be host[:port], not a URL")
	}
	if strings.TrimSpace(cfg.username) == "" {
		return config{}, "", errors.New("--username must not be empty")
	}
	if cfg.oidcLifetime <= 0 {
		return config{}, "", errors.New("--oidc-lifetime must be greater than zero")
	}

	return cfg, operation, nil
}

func runGet(cfg config, stdin io.Reader, stdout, stderr io.Writer) int {
	request, err := gitcred.ParseRequest(stdin)
	if err != nil {
		writeStderr(stderr, "parse git credential request: %v\n", err)
		return 1
	}
	if err := validateRequest(request, cfg.allowedAuthority); err != nil {
		writeStderr(stderr, "%v\n", err)
		return 1
	}

	jobID := os.Getenv("BUILDKITE_JOB_ID")
	credentialCache, cacheKey, err := prepareCache(cfg, request, jobID)
	if err != nil {
		writeStderr(stderr, "%v\n", err)
		return 1
	}

	now := time.Now()
	_ = credentialCache.CleanupExpired(now)

	entry, ok, err := credentialCache.Get(cacheKey, now)
	if err != nil {
		writeStderr(stderr, "read credential cache: %v\n", err)
		return 1
	}
	if ok {
		return writeGitResponse(stdout, gitcred.Response{
			Username:          entry.Username,
			Password:          entry.Password,
			PasswordExpiryUTC: entry.PasswordExpiryUTC,
		}, stderr)
	}

	oidcClient, err := buildkiteoidc.NewFromEnv(nil)
	if err != nil {
		writeStderr(stderr, "configure Buildkite OIDC client: %v\n", err)
		return 1
	}

	token, err := oidcClient.RequestToken(context.Background(), cfg.audience, cfg.oidcLifetime)
	if err != nil {
		writeStderr(stderr, "request Buildkite OIDC token: %v\n", err)
		return 1
	}

	exchangeClient, err := exchange.New(cfg.exchangeURL, nil)
	if err != nil {
		writeStderr(stderr, "configure token exchange client: %v\n", err)
		return 1
	}

	exchanged, err := exchangeClient.Exchange(context.Background(), token, exchange.Request{
		Protocol:  request.Protocol,
		Authority: request.Authority,
		Path:      request.Path,
	})
	if err != nil {
		writeStderr(stderr, "exchange Git credential: %v\n", err)
		return 1
	}

	entry = cache.Entry{
		JobID:             jobID,
		Username:          cfg.username,
		Password:          exchanged.Password,
		PasswordExpiryUTC: exchanged.PasswordExpiryUTC,
	}
	if err := credentialCache.Put(cacheKey, entry); err != nil {
		writeStderr(stderr, "write credential cache: %v\n", err)
		return 1
	}

	return writeGitResponse(stdout, gitcred.Response{
		Username:          cfg.username,
		Password:          exchanged.Password,
		PasswordExpiryUTC: exchanged.PasswordExpiryUTC,
	}, stderr)
}

func runErase(cfg config, stdin io.Reader, stderr io.Writer) int {
	request, err := gitcred.ParseRequest(stdin)
	if err != nil {
		writeStderr(stderr, "parse git credential request: %v\n", err)
		return 1
	}
	if err := validateRequest(request, cfg.allowedAuthority); err != nil {
		writeStderr(stderr, "%v\n", err)
		return 1
	}

	jobID := os.Getenv("BUILDKITE_JOB_ID")
	credentialCache, cacheKey, err := prepareCache(cfg, request, jobID)
	if err != nil {
		writeStderr(stderr, "%v\n", err)
		return 1
	}
	if err := credentialCache.Erase(cacheKey); err != nil {
		writeStderr(stderr, "erase credential cache: %v\n", err)
		return 1
	}

	return 0
}

func prepareCache(cfg config, request gitcred.Request, jobID string) (*cache.Cache, string, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, "", errors.New("BUILDKITE_JOB_ID is required")
	}
	credentialCache, err := cache.New(cfg.cacheBaseDir, jobID, cacheRefreshSkew)
	if err != nil {
		return nil, "", fmt.Errorf("create credential cache: %w", err)
	}
	cachePath := repoid.NormalizeForCache(request.Path)
	return credentialCache, credentialCacheKey(request.Protocol, request.Authority, cachePath, cfg.audience), nil
}

func validateRequest(request gitcred.Request, allowedAuthority string) error {
	if !strings.EqualFold(request.Protocol, "https") {
		return fmt.Errorf("unsupported git credential protocol %q: HTTPS is required", request.Protocol)
	}
	if strings.TrimSpace(request.Authority) == "" {
		return errors.New("git credential request missing authority")
	}
	if !strings.EqualFold(request.Authority, allowedAuthority) {
		return fmt.Errorf("git credential authority %q does not match configured authority %q", request.Authority, allowedAuthority)
	}
	if repoid.NormalizeForCache(request.Path) == "" {
		return errors.New("git credential request missing path")
	}
	return nil
}

func credentialCacheKey(protocol, authority, path, audience string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.ToLower(protocol),
		strings.ToLower(authority),
		path,
		audience,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func writeGitResponse(stdout io.Writer, response gitcred.Response, stderr io.Writer) int {
	if err := response.Write(stdout); err != nil {
		writeStderr(stderr, "write git credential response: %v\n", err)
		return 1
	}
	return 0
}

func writeStderr(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, format, args...)
}
