package job

import (
	"context"
	"errors"

	"github.com/ThreeDotsLabs/watermill/message"
)

var ErrPermanent = errors.New("permanent queue job failure")

type Job interface {
	GetJobName() string
	GetJobTopic() string
	GetJobData() (message.Payload, error)
	ParseJob(data message.Payload) (Job, error)
	RunJob(ctx context.Context) error
}
