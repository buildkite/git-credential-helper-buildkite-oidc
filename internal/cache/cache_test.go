package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheExpiresEntriesWithSkew(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	credentialCache, err := New(cacheDir, "job-123", 30*time.Second)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	key := keyForTest("repo")
	now := time.Unix(1_700_000_000, 0)
	if err := credentialCache.Put(key, Entry{
		JobID:             "job-123",
		Username:          "buildkite-agent",
		Password:          "secret",
		PasswordExpiryUTC: now.Add(20 * time.Second).Unix(),
	}); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	_, ok, err := credentialCache.Get(key, now)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("expected expired entry to be ignored")
	}
}

func TestCacheEraseRemovesEntry(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	credentialCache, err := New(cacheDir, "job-123", 30*time.Second)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	key := keyForTest("repo")
	if err := credentialCache.Put(key, Entry{
		JobID:             "job-123",
		Username:          "buildkite-agent",
		Password:          "secret",
		PasswordExpiryUTC: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	if err := credentialCache.Erase(key); err != nil {
		t.Fatalf("Erase returned error: %v", err)
	}

	_, ok, err := credentialCache.Get(key, time.Now())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("expected erased entry to be missing")
	}
}

func keyForTest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
