package queue

import (
	"errors"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/caitunai/go-blueprint/queue/job"
	"github.com/spf13/viper"
)

var (
	ErrPublishTopicMessage = errors.New("publish message to topic failed")
	ErrPublisherNotReady   = errors.New("queue publisher is not initialized")
)

func Publish(j job.Job) error {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	if publisher == nil {
		return ErrPublisherNotReady
	}
	topic := j.GetJobTopic()
	if topic == "" {
		topic = "default"
	}
	m := make(message.Metadata)
	m.Set("name", j.GetJobName())
	topicPrefix := viper.GetString("queue.prefix")
	if err := publisher.Publish(topicPrefix+":"+topic, &message.Message{
		UUID:     watermill.NewUUID(),
		Metadata: m,
		Payload:  j.GetJobData(),
	}); err != nil {
		return errors.Join(ErrPublishTopicMessage, err)
	}
	return nil
}
