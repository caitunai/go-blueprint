package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/caitunai/go-blueprint/queue/job"
	"github.com/caitunai/go-blueprint/redis"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const defaultQueueShutdownTimeout = 5 * time.Second

var (
	publisher               *redisstream.Publisher
	subscriber              *redisstream.Subscriber
	clientMutex             sync.RWMutex
	jobs                    = make(map[string]job.Job)
	ErrRedisStream          = errors.New("create redis stream client failed")
	ErrCloseRedisStream     = errors.New("close redis stream client failed")
	ErrQueueShutdownTimeout = errors.New("queue shutdown timeout")
	ErrJobHandlerNotFound   = errors.New("queue job handler not found")
	ErrRunJob               = errors.New("run queue job failed")
)

func Init() error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	if publisher != nil {
		return nil
	}
	var err error
	publisher, err = redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:     redis.GetBorrowedClient(),
			Marshaller: redisstream.DefaultMarshallerUnmarshaller{},
		},
		NewLogger(),
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("method", "Init").
			Str("package", "queue").
			Msg("create queue publisher failed")
		return errors.Join(ErrRedisStream, err)
	}
	return nil
}

func Start(ctx context.Context, subscriberID string) error {
	streamSubscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:        redis.GetBorrowedClient(),
			Consumer:      viper.GetString("queue.consumer.name") + subscriberID,
			ConsumerGroup: viper.GetString("queue.consumer.group"),
		},
		NewLogger(),
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("method", "Init").
			Str("package", "queue").
			Msg("create queue subscriber failed")
		return errors.Join(ErrRedisStream, err)
	}
	clientMutex.Lock()
	subscriber = streamSubscriber
	clientMutex.Unlock()

	SubscribeJob()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	consumeCtx, cancelConsume := context.WithCancel(ctx)
	jobCtx, cancelJobs := context.WithCancel(ctx)
	stopConsuming := make(chan struct{})
	var listeners sync.WaitGroup
	subscribe(consumeCtx, jobCtx, stopConsuming, streamSubscriber, quit, &listeners)

	var shutdownReason string
	select {
	case sig := <-quit:
		shutdownReason = sig.String()
	case <-ctx.Done():
		shutdownReason = ctx.Err().Error()
	}
	close(stopConsuming)

	shutdownErr := waitForListeners(&listeners, queueShutdownTimeout())
	if shutdownErr != nil {
		cancelJobs()
	}
	cancelConsume()
	cancelJobs()
	closeErr := Close()
	log.Info().Str("reason", shutdownReason).Msg("stopped queue jobs")
	return errors.Join(shutdownErr, closeErr)
}

func queueShutdownTimeout() time.Duration {
	timeout := viper.GetDuration("queue.shutdownTimeout")
	if timeout <= 0 {
		return defaultQueueShutdownTimeout
	}
	return timeout
}

func waitForListeners(listeners *sync.WaitGroup, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		listeners.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return ErrQueueShutdownTimeout
	}
}

func subscribe(
	consumeCtx context.Context,
	jobCtx context.Context,
	stop <-chan struct{},
	streamSubscriber *redisstream.Subscriber,
	kill chan<- os.Signal,
	listeners *sync.WaitGroup,
) {
	topics := viper.GetStringSlice("queue.topics")
	if len(topics) == 0 {
		listeners.Add(1)
		go listenTopic(consumeCtx, jobCtx, stop, streamSubscriber, "default", kill, listeners)
	} else {
		for _, topic := range topics {
			listeners.Add(1)
			go listenTopic(consumeCtx, jobCtx, stop, streamSubscriber, topic, kill, listeners)
		}
	}
}

func listenTopic(
	consumeCtx context.Context,
	jobCtx context.Context,
	stop <-chan struct{},
	streamSubscriber *redisstream.Subscriber,
	topic string,
	kill chan<- os.Signal,
	listeners *sync.WaitGroup,
) {
	defer func() {
		if e := recover(); e != nil {
			log.Error().
				Str("topic", topic).
				Str("reason", fmt.Sprintf("recover: %v", e)).
				Bytes("stack", debug.Stack()).
				Msg("panic when listen topic")
			select {
			case kill <- syscall.Signal(-10000):
			default:
			}
		}
		listeners.Done()
	}()
	topicPrefix := viper.GetString("queue.prefix")
	messages, err := streamSubscriber.Subscribe(consumeCtx, topicPrefix+":"+topic)
	if err != nil {
		log.Error().
			Err(err).
			Str("method", "listenTopic").
			Str("package", "queue").
			Msg("listen topic messages failed")
		return
	}
	log.Info().
		Str("topic", topic).
		Str("prefix", topicPrefix).
		Msg("subscribe topic successfully")
	for {
		select {
		case <-stop:
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}
			if err := dispatch(jobCtx, topic, msg.Metadata.Get("name"), msg.UUID, msg.Payload); err != nil &&
				jobCtx.Err() != nil {
				return
			}
			msg.Ack()
		}
	}
}

func dispatch(ctx context.Context, topic, name, id string, data message.Payload) error {
	j, ok := jobs[name]
	if !ok {
		log.Error().
			Str("topic", topic).
			Str("job_name", name).
			Bytes("data", data).
			Msg("job handler not found")
		return ErrJobHandlerNotFound
	}
	if err := j.ParseJob(data).RunJob(ctx); err != nil {
		log.Error().
			Err(err).
			Str("topic", topic).
			Str("job_name", name).
			Bytes("data", data).
			Msg("job handler run failed")
		return errors.Join(ErrRunJob, err)
	}
	log.Info().
		Str("topic", topic).
		Str("name", name).
		Str("job_id", id).
		Msg("run job successfully")
	return nil
}

// Close stops Watermill clients without closing the shared Redis connection.
func Close() error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	streamSubscriber := subscriber
	streamPublisher := publisher
	subscriber = nil
	publisher = nil

	var subscriberErr error
	if streamSubscriber != nil {
		subscriberErr = streamSubscriber.Close()
	}
	var publisherErr error
	if streamPublisher != nil {
		publisherErr = streamPublisher.Close()
	}
	closeErr := errors.Join(subscriberErr, publisherErr)
	if closeErr != nil {
		return errors.Join(ErrCloseRedisStream, closeErr)
	}
	return nil
}

func register(j job.Job) {
	jobs[j.GetJobName()] = j
}
