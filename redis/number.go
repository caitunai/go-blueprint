package redis

import (
	"context"
	"time"
)

func Increment(ctx context.Context, key string, t time.Duration) (int64, error) {
	res := rdb.Incr(ctx, key)
	result, err := res.Result()
	if err != nil {
		return 0, err
	}
	rdb.Expire(ctx, key, t)
	return result, nil
}

func Decrement(ctx context.Context, key string, t time.Duration) (int64, error) {
	res := rdb.Decr(ctx, key)
	result, err := res.Result()
	if err != nil {
		return 0, err
	}
	rdb.Expire(ctx, key, t)
	return result, nil
}
