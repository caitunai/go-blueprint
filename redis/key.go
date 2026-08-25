package redis

import (
	"errors"
	"time"

	"github.com/spf13/viper"
)

var (
	ErrInvalidKey        = errors.New("redis key is empty")
	ErrInvalidExpiration = errors.New("redis expiration must be positive")
	ErrKeyNotFound       = errors.New("redis key not found")
)

func WithPrefix(k string) string {
	prefix := viper.GetString("redis.prefix")
	if prefix != "" {
		return prefix + ":" + k
	}
	return k
}

func prefixedKey(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}
	return WithPrefix(key), nil
}

func validateExpiration(expiration time.Duration) error {
	if expiration < time.Millisecond {
		return ErrInvalidExpiration
	}
	return nil
}
