package disk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Cache struct {
	cacheDir string
}

func NewDiskCache() (*Cache, error) {
	return &Cache{
		cacheDir: "/tmp/sippycache",
	}, nil
}

// GenKeyFilename generates a SHA-256 hash of the input string, and uses it to return a filename
// we can use for this cache key on disk.
func (c Cache) GenKeyFilename(input string) string {
	hash := sha256.Sum256([]byte(input))
	hexStr := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s/%s.json", c.cacheDir, hexStr)
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
	return data, nil
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

	err := os.WriteFile(filename, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to cache file: %v", err)
	}
	return nil
}
