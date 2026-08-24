package redis

import (
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var (
	rdb                *redis.Client
	rdbMutex           sync.Mutex
	ErrCloseConnection = errors.New("close redis connection failed")
)

type borrowedClient struct {
	redis.UniversalClient
}

// Close leaves the shared client open because the redis package owns it.
func (borrowedClient) Close() error {
	return nil
}

func Init() {
	rdbMutex.Lock()
	defer rdbMutex.Unlock()

	if rdb != nil {
		return
	}
	rdb = newClient()
}

func newClient() *redis.Client {
	addr := viper.GetString("redis.host") + ":" + viper.GetString("redis.port")
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})
}

func GetClient() *redis.Client {
	rdbMutex.Lock()
	defer rdbMutex.Unlock()

	if rdb == nil {
		rdb = newClient()
	}
	return rdb
}

// GetBorrowedClient returns the shared Redis client without transferring
// ownership. Calling Close on the returned client does not close the pool.
func GetBorrowedClient() redis.UniversalClient {
	return borrowedClient{UniversalClient: GetClient()}
}

// Close disconnects every connection in the shared Redis client pool.
// Callers must stop Redis users before closing the shared client.
func Close() error {
	rdbMutex.Lock()
	defer rdbMutex.Unlock()

	if rdb == nil {
		return nil
	}
	client := rdb
	rdb = nil
	if err := client.Close(); err != nil {
		return errors.Join(ErrCloseConnection, err)
	}
	return nil
}
