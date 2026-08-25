package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrPushListRight   = errors.New("error push right to list")
	ErrGetListElements = errors.New("error get all list elements")
	ErrInvalidList     = errors.New("invalid redis list operation")
)

const maxListPushValues = 1000

func RightPushWithLimitExpired(ctx context.Context, key string, values []string, limit int64, expired time.Duration) error {
	redisKey, err := prefixedKey(key)
	if err != nil || len(values) == 0 || len(values) > maxListPushValues || limit <= 0 {
		return errors.Join(ErrPushListRight, ErrInvalidList, err)
	}
	if err := validateExpiration(expired); err != nil {
		return errors.Join(ErrPushListRight, err)
	}
	params := make([]any, len(values))
	for i, value := range values {
		params[i] = value
	}
	_, err = GetClient().TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.RPush(ctx, redisKey, params...)
		pipe.LTrim(ctx, redisKey, -limit, -1)
		pipe.PExpire(ctx, redisKey, expired)
		return nil
	})
	if err != nil {
		return errors.Join(ErrPushListRight, err)
	}
	return nil
}

func GetListAllElements(ctx context.Context, key string, result *[]string) error {
	redisKey, err := prefixedKey(key)
	if err != nil || result == nil {
		return errors.Join(ErrGetListElements, ErrInvalidList, err)
	}
	list := GetClient().LRange(ctx, redisKey, 0, -1)
	err = list.ScanSlice(result)
	if err != nil {
		return errors.Join(ErrGetListElements, err)
	}
	return nil
}
