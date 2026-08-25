package cache

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	projectredis "github.com/caitunai/go-blueprint/redis"
	"github.com/spf13/viper"
)

func TestCacheIntegration(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	Close()
	if err := projectredis.Close(); err != nil {
		t.Fatalf("redis.Close() error = %v", err)
	}
	viper.Set("redis.host", host)
	viper.Set("redis.port", port)
	viper.Set("redis.db", 14)
	viper.Set("redis.prefix", "cache-integration")
	ctx := context.Background()
	if err := projectredis.Init(ctx); err != nil {
		t.Fatalf("redis.Init() error = %v", err)
	}
	if err := projectredis.GetClient().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("FlushDB() error = %v", err)
	}
	InitCache()
	t.Cleanup(func() {
		Close()
		_ = projectredis.GetClient().FlushDB(ctx).Err()
		_ = projectredis.Close()
	})

	if err := PutString(ctx, "key", "value", time.Minute); err != nil {
		t.Fatalf("PutString() error = %v", err)
	}
	value, err := GetString(ctx, "key")
	if err != nil || value != "value" {
		t.Fatalf("GetString() = %q, %v, want value, nil", value, err)
	}
	if err := Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := GetString(ctx, "key"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("GetString(deleted) error = %v, want ErrCacheMiss", err)
	}
}
