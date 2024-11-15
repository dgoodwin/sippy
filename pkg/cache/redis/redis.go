package redis

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	r "gopkg.in/redis.v5"
)

const prefix = "_SIPPY_"

type Cache struct {
	client *r.Client
}

func NewRedisCache(url string) (*Cache, error) {
	var opts *r.Options
	var err error

	if opts, err = r.ParseURL(url); err != nil {
		return nil, err
	}

	return &Cache{
		client: r.NewClient(opts),
	}, nil
}

func (c Cache) Get(_ context.Context, key string) ([]byte, error) {
	before := time.Now()
	defer func(key string, before time.Time) {
		logrus.Infof("Redis Cache Get completed in %s for %s", time.Since(before), key)
	}(key, before)
	return c.client.Get(prefix + key).Bytes()
}

func (c Cache) Set(_ context.Context, key string, content []byte, duration time.Duration) error {
	before := time.Now()
	defer func(key string, before time.Time) {
		logrus.Infof("Redis Cache Set completed in %s for %s", time.Since(before), key)
	}(key, before)
	return c.client.Set(prefix+key, content, duration).Err()
}
