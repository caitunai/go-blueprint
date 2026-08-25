package queue

import (
	"context"
	"errors"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/caitunai/go-blueprint/queue/job"
	"github.com/spf13/viper"
)

var (
	ErrPublishTopicMessage = errors.New("publish message to topic failed")
	ErrPublisherNotReady   = errors.New("queue publisher is not initialized")
	ErrEncodeJob           = errors.New("encode queue job failed")
	ErrInvalidJob          = errors.New("queue job is nil")
)

func Publish(ctx context.Context, j job.Job) error {
	if j == nil {
		return errors.Join(ErrPublishTopicMessage, ErrInvalidJob)
	}
	payload, err := j.GetJobData()
	if err != nil {
		return errors.Join(ErrPublishTopicMessage, ErrEncodeJob, err)
	}
	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.Metadata.Set("name", j.GetJobName())
	msg.SetContext(ctx)
	return publishMessage(j.GetJobTopic(), msg)
}

func publishMessage(topic string, msg *message.Message) error {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	if publisher == nil {
		return ErrPublisherNotReady
	}
	if topic == "" {
		topic = "default"
	}
	if err := publisher.Publish(queueTopic(topic), msg); err != nil {
		return errors.Join(ErrPublishTopicMessage, err)
	}
	return nil
}

func queueTopic(topic string) string {
	topicPrefix := viper.GetString("queue.prefix")
	if topicPrefix == "" {
		return topic
	}
	return topicPrefix + ":" + topic
}
