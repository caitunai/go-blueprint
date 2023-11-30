package queue

import "github.com/caitunai/go-blueprint/queue/job"

func SubscribeJob() {
	register(&job.Example{})
}
