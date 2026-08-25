package queue

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/caitunai/go-blueprint/queue/job"
	projectredis "github.com/caitunai/go-blueprint/redis"
	"github.com/spf13/viper"
)

func TestPublisherBoundsStreamLengthIntegration(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	viper.Set("redis.host", host)
	viper.Set("redis.port", port)
	viper.Set("redis.db", 15)
	viper.Set("queue.prefix", "queue-integration")
	viper.Set("queue.streamMaxLength", 1000)
	viper.Set("queue.deadLetterMaxLength", 10)
	ctx := context.Background()
	if err := projectredis.Init(ctx); err != nil {
		t.Fatalf("redis.Init() error = %v", err)
	}
	if err := projectredis.GetClient().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("FlushDB() error = %v", err)
	}
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() {
		_ = Close()
		_ = projectredis.GetClient().FlushDB(ctx).Err()
		_ = projectredis.Close()
	})

	for number := range 500 {
		if err := Publish(ctx, &job.Example{Number: number}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	length, err := projectredis.GetClient().XLen(ctx, queueTopic("default")).Result()
	if err != nil {
		t.Fatalf("XLen() error = %v", err)
	}
	if length != 500 {
		t.Fatalf("ordinary stream length = %d, want 500", length)
	}

	for range 500 {
		deadLetter := message.NewMessage(watermill.NewUUID(), []byte("failed"))
		deadLetter.SetContext(ctx)
		if err := publishMessage("default."+deadLetterSuffix(), deadLetter); err != nil {
			t.Fatalf("publishMessage(DLQ) error = %v", err)
		}
	}
	deadLetterLength, err := projectredis.GetClient().XLen(
		ctx,
		queueTopic("default."+deadLetterSuffix()),
	).Result()
	if err != nil {
		t.Fatalf("XLen(DLQ) error = %v", err)
	}
	if deadLetterLength > 200 {
		t.Fatalf("DLQ stream length = %d, want at most 200 after approximate trim", deadLetterLength)
	}
}
