package redis

import (
	"context"
	"errors"
	"time"
)

var ErrSetExpire = errors.New("set redis key expiration failed")

// SetExpire sets a relative expiration on a prefixed application key.
func SetExpire(ctx context.Context, key string, expiration time.Duration) error {
	redisKey, err := prefixedKey(key)
	if err != nil {
		return errors.Join(ErrSetExpire, err)
	}
	if err := validateExpiration(expiration); err != nil {
		return errors.Join(ErrSetExpire, err)
	}
	exists, err := GetClient().PExpire(ctx, redisKey, expiration).Result()
	if err != nil {
		return errors.Join(ErrSetExpire, err)
	}
	if !exists {
		return errors.Join(ErrSetExpire, ErrKeyNotFound)
	}
	return nil
}
