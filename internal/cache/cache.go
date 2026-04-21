package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Entry struct {
	JobID             string `json:"job_id"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	PasswordExpiryUTC int64  `json:"password_expiry_utc"`
}

type Cache struct {
	dir  string
	skew time.Duration
}

func DefaultBaseDir() string {
	return filepath.Join(os.TempDir(), "git-credential-buildkite-oidc")
}

func New(baseDir, jobID string, skew time.Duration) (*Cache, error) {
	if jobID == "" {
		return nil, errors.New("missing job ID")
	}
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	dir := filepath.Join(baseDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("set cache directory permissions: %w", err)
	}
	return &Cache{dir: dir, skew: skew}, nil
}

func (c *Cache) Get(key string, now time.Time) (Entry, bool, error) {
	var entry Entry
	err := c.withLock(key, func() error {
		payload, err := os.ReadFile(c.entryPath(key))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read cache file: %w", err)
		}
		if err := json.Unmarshal(payload, &entry); err != nil {
			return fmt.Errorf("decode cache file: %w", err)
		}
		if c.expired(entry, now) {
			if err := os.Remove(c.entryPath(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove expired cache file: %w", err)
			}
			entry = Entry{}
		}
		return nil
	})
	if err != nil {
		return Entry{}, false, err
	}
	if entry == (Entry{}) {
		return Entry{}, false, nil
	}
	return entry, true, nil
}

func (c *Cache) Put(key string, entry Entry) error {
	if entry.JobID == "" {
		return errors.New("cache entry missing job ID")
	}
	if entry.Password == "" {
		return errors.New("cache entry missing password")
	}
	if entry.PasswordExpiryUTC == 0 {
		return errors.New("cache entry missing password expiry")
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode cache entry: %w", err)
	}

	return c.withLock(key, func() error {
		tempFile, err := os.CreateTemp(c.dir, key+"-*.tmp")
		if err != nil {
			return fmt.Errorf("create temporary cache file: %w", err)
		}
		tempPath := tempFile.Name()
		defer func() {
			_ = os.Remove(tempPath)
		}()

		if err := tempFile.Chmod(0o600); err != nil {
			_ = tempFile.Close()
			return fmt.Errorf("set cache file permissions: %w", err)
		}
		if _, err := tempFile.Write(payload); err != nil {
			_ = tempFile.Close()
			return fmt.Errorf("write cache file: %w", err)
		}
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("close cache file: %w", err)
		}
		if err := os.Rename(tempPath, c.entryPath(key)); err != nil {
			return fmt.Errorf("replace cache file: %w", err)
		}
		return nil
	})
}

func (c *Cache) Erase(key string) error {
	return c.withLock(key, func() error {
		if err := os.Remove(c.entryPath(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove cache file: %w", err)
		}
		return nil
	})
}

func (c *Cache) CleanupExpired(now time.Time) error {
	entries, err := os.ReadDir(c.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(c.dir, entry.Name())
		payload, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record Entry
		if err := json.Unmarshal(payload, &record); err != nil {
			continue
		}
		if !c.expired(record, now) {
			continue
		}
		_ = os.Remove(path)
	}

	return nil
}

func (c *Cache) entryPath(key string) string {
	return filepath.Join(c.dir, key+".json")
}

func (c *Cache) lockPath(key string) string {
	return filepath.Join(c.dir, key+".lock")
}

func (c *Cache) expired(entry Entry, now time.Time) bool {
	expiresAt := time.Unix(entry.PasswordExpiryUTC, 0).Add(-c.skew)
	return entry.JobID == "" || now.After(expiresAt)
}

func (c *Cache) withLock(key string, fn func() error) error {
	lockFile, err := os.OpenFile(c.lockPath(key), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer func() {
		_ = lockFile.Close()
	}()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock cache file: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	return fn()
}
