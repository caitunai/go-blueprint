package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strconv"
)

func AddSortItem(ctx context.Context, key, item string, value int64) error {
	res := rdb.ZAdd(ctx, key, redis.Z{
		Score:  float64(value),
		Member: item,
	})
	return res.Err()
}

// RemoveSortItem remove items include min, max value
func RemoveSortItem(ctx context.Context, key string, min, max int64) error {
	res := rdb.ZRemRangeByScore(ctx, key, strconv.FormatInt(min, 10), strconv.FormatInt(max, 10))
	return res.Err()
}

func GetMinSortItem(ctx context.Context, key string) (string, error) {
	res := rdb.ZRange(ctx, key, 0, 0)
	result, err := res.Result()
	if err != nil {
		return "", err
	}
	if len(result) > 0 {
		return result[0], nil
	}
	return "", err
}
