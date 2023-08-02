package redis

import (
	"context"
	"time"
)

func RightPushWithLimitExpired(ctx context.Context, key string, values []string, limit int64, expired time.Duration) error {
	pipe := rdb.Pipeline()
	params := make([]any, len(values), len(values))
	for i, value := range values {
		params[i] = value
	}
	pipe.RPush(
		ctx,
		key,
		params...,
	)
	pipe.LTrim(ctx, key, 0-limit, -1)
	pipe.Expire(ctx, key, expired)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func GetListAllElements(ctx context.Context, key string, result *[]string) error {
	list := rdb.LRange(ctx, key, 0, -1)
	err := list.ScanSlice(result)
	if err != nil {
		return err
	}
	return nil
}
