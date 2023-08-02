package cache

import (
	"context"
	"errors"
	"github.com/caitunai/go-blueprint/redis"
	"github.com/go-redis/cache/v9"
	"time"
)

var (
	cli             *cache.Cache
	ErrorConnection = errors.New("error connection")
)

func InitCache() {
	cli = cache.New(&cache.Options{
		Redis: redis.GetClient(),
	})
}

func GetClient() *cache.Cache {
	return cli
}

func PutString(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := cli.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: value,
		TTL:   ttl,
	}); err != nil {
		return err
	}
	return nil
}

func GetString(ctx context.Context, key string) (string, error) {
	var wanted string
	err := cli.Get(ctx, key, &wanted)
	if err == nil {
		return wanted, nil
	}
	return wanted, err
}
