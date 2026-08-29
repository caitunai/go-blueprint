package queue

import "github.com/caitunai/go-blueprint/queue/job"

// SubscribeJob performs the subscribe job operation.
func SubscribeJob() {
	register(&job.Example{})
}
