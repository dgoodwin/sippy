package disk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

const (
	cacheExpiryScanInterval = 1 * time.Minute
)

type Cache struct {
	cacheDir string
	logger   *logrus.Logger
}

func NewDiskCache() (*Cache, error) {
	// Start the cleanup goroutine
	cache := &Cache{
		cacheDir: "/tmp/sippycache",
		logger:   logrus.WithField("cache", "disk"),
	}
	go cache.cleanupExpiredCacheFiles()
	return cache, nil
}

// GenKeyFilename generates a SHA-256 hash of the input string, and uses it to return a filename
// we can use for this cache key on disk. It will have the same prefix as the cache key when possible
// for easier debugging.
func (c Cache) GenKeyFilename(input string) string {
	hash := sha256.Sum256([]byte(input))
	hexStr := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s/%s%s.json", c.cacheDir, ExtractPrefix(input), hexStr)
}

// ExtractPrefix extracts the prefix from a string before the '~' character.
// If the key does not contain a prefix we expect, we return empty string.
func ExtractPrefix(input string) string {
	if idx := strings.Index(input, "~"); idx != -1 {
		return fmt.Sprintf("%s-", input[:idx])
	}
	return ""
}

func (c Cache) Get(_ context.Context, key string, _ time.Duration) ([]byte, error) {
	before := time.Now()
	filename := c.GenKeyFilename(key)
	defer func(filename, key string, before time.Time) {
		logrus.Infof("Disk Cache Get completed in %s for %s (file: %s)", time.Since(before), key, filename)
	}(filename, key, before)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("cache file %s does not exist", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading cache file %s", filename)
	}
	logrus.Infof("returned cache file data from %s", filename)
	return data, nil
}

type cacheKeyFileMetadata struct {
	Key      string    `json:"key"`
	Filename string    `json:"filename"`
	Set      time.Time `json:"set"`
	Expire   time.Time `json:"expire"`
}

func (c Cache) Set(_ context.Context, key string, content []byte, duration time.Duration) error {
	before := time.Now()
	filename := c.GenKeyFilename(key)
	defer func(filename, key string, before time.Time) {
		logrus.Infof("Disk Cache Set completed in %s for %s (file: %s)", time.Since(before), key, filename)
	}(filename, key, before)

	// create our cache directory if it doesn't exist
	if _, err := os.Stat(c.cacheDir); os.IsNotExist(err) {
		err := os.MkdirAll(c.cacheDir, 0755)
		if err != nil {
			return fmt.Errorf("cache directory %s does not exist, unable to create; %s", c.cacheDir, err)
		}
		logrus.Infof("cache directory %s created", c.cacheDir)
	}

	// Create metadata
	metadata := cacheKeyFileMetadata{
		Key:      key,
		Filename: filename,
		Set:      before,
		Expire:   before.Add(duration),
	}

	// metadata filename, this will contain details on the original key, set time and expiry time
	metadataFilename := fmt.Sprintf("%s-metadata.json", filename[:len(filename)-len(".json")])

	// file lock to prevent multiple goroutines or another sippy replica from conflicting
	lock := flock.New(filename)
	locked, err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("error attempting to acquire file lock: %v", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire lock on file: %s", filename)
	}
	defer lock.Unlock()

	metadataContent, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %v", err)
	}

	// Write cache content to temporary file
	tempFilename := fmt.Sprintf("%s.tmp", filename)
	if err := os.WriteFile(tempFilename, content, 0644); err != nil {
		return fmt.Errorf("failed to write to temporary cache file: %v", err)
	}

	tempMetadataFilename := fmt.Sprintf("%s.tmp", metadataFilename)
	if err := os.WriteFile(tempMetadataFilename, metadataContent, 0644); err != nil {
		return fmt.Errorf("failed to write temporary metadata file: %v", err)
	}

	// Atomically rename temporary files to their final names
	if err := os.Rename(tempFilename, filename); err != nil {
		return fmt.Errorf("failed to rename temporary cache file: %v", err)
	}

	if err := os.Rename(tempMetadataFilename, metadataFilename); err != nil {
		return fmt.Errorf("failed to rename temporary metadata file: %v", err)
	}

	return nil
}

// cleanupExpiredCacheFiles periodically checks all metadata files, and deletes expired cache files and metadata if
// we're past their expiration time.
func (c *Cache) cleanupExpiredCacheFiles() {
	ticker := time.NewTicker(cacheExpiryScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup performs the actual cleanup of expired files. Uses the same file lock to prevent concurrent writes with
// Set or another sippy replica.
func (c *Cache) cleanup() {
	logrus.Info("cleaning up expired cache files")
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		logrus.Errorf("failed to read cache directory %s: %v", c.cacheDir, err)
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "-metadata.json") {
			// read and parse metadata file to see if we're past our expiration time:
			metadataFilename := filepath.Join(c.cacheDir, file.Name())
			metadataContent, err := os.ReadFile(metadataFilename)
			if err != nil {
				logrus.Errorf("failed to read metadata file %s: %v", metadataFilename, err)
				continue
			}

			var metadata cacheKeyFileMetadata
			if err := json.Unmarshal(metadataContent, &metadata); err != nil {
				logrus.Errorf("failed to unmarshal metadata from file %s: %v", metadataFilename, err)
				continue
			}

			logrus.Infof("checking cache metadata file for expiry: %s", metadataFilename)

			// if expired, delete cache file and metadata file
			if time.Now().After(metadata.Expire) {
				logrus.Infof("cache metadata file %s expired, deleting it and the data file", metadataFilename)
				cacheFile := metadata.Filename

				// we just lock on the actual cache data file on the assumption the metadata file is only
				// ever written when the data file is locked.
				cacheLock := flock.New(cacheFile + ".lock")
				lockFile := cacheFile + ".lock"

				cacheLocked, err := cacheLock.TryLock()
				if err != nil {
					logrus.Errorf("failed to lock cache file %s: %v", cacheFile, err)
					continue
				}
				if !cacheLocked {
					logrus.Infof("cache file %s is locked by another process, skipping cleanup.", cacheFile)
					continue
				}
				defer func() {
					// Unlock won't delete the lock file itself.
					cacheLock.Unlock()
					if err := os.Remove(lockFile); err != nil {
						logrus.Errorf("failed to delete lock file %s: %v", lockFile, err)
					} else {
						logrus.Infof("lock file %s deleted successfully", lockFile)
					}
				}()

				logrus.Infof("deleting expired cache file %s and its metadata", cacheFile)

				// remove cache file first, then the metadata
				if err := os.Remove(cacheFile); err != nil {
					logrus.Errorf("failed to delete expired cache file %s: %v", cacheFile, err)
				}
				if err := os.Remove(metadataFilename); err != nil {
					logrus.Errorf("failed to delete expired metadata file %s: %v", metadataFilename, err)
				}
			}
		}
	}
	logrus.Info("cache cleanup complete")
}
