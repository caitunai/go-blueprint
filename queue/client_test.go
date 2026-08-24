package queue

import (
	"errors"
	"testing"

	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/caitunai/go-blueprint/queue/job"
	"github.com/caitunai/go-blueprint/redis"
)

func TestCloseWatermillClientsKeepsSharedRedisOwnedByRedisPackage(t *testing.T) {
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close() cleanup error = %v", err)
		}
		if err := redis.Close(); err != nil {
			t.Errorf("redis.Close() cleanup error = %v", err)
		}
	})

	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	streamSubscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{Client: redis.GetBorrowedClient()},
		NewLogger(),
	)
	if err != nil {
		t.Fatalf("NewSubscriber() error = %v", err)
	}
	clientMutex.Lock()
	subscriber = streamSubscriber
	clientMutex.Unlock()

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := redis.Close(); err != nil {
		t.Fatalf("redis.Close() error = %v", err)
	}
	if err := Publish(&job.Example{}); !errors.Is(err, ErrPublisherNotReady) {
		t.Fatalf("Publish() error = %v, want ErrPublisherNotReady", err)
	}
}
