package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
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

func TestCacheRejectsEntryForDifferentJob(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	credentialCache, err := New(cacheDir, "job-123", 30*time.Second)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	key := keyForTest("repo")
	payload, err := json.Marshal(Entry{
		JobID:             "job-456",
		Username:          "buildkite-agent",
		Password:          "secret",
		PasswordExpiryUTC: time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(credentialCache.entryPath(key), payload, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, ok, err := credentialCache.Get(key, time.Now())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("expected mismatched job entry to be ignored")
	}
	if _, err := os.Stat(credentialCache.entryPath(key)); !os.IsNotExist(err) {
		t.Fatalf("expected mismatched job entry to be removed, stat error: %v", err)
	}
}

func keyForTest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
