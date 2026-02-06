package sqs

import "github.com/sachamama/sacha/internal/sqs"

// queuesLoadedMsg is sent when the initial queue listing completes.
type queuesLoadedMsg struct {
	queues    []sqs.Queue
	nextToken *string
	err       error
}

// moreQueuesLoadedMsg is sent when additional queues are loaded (lazy loading).
type moreQueuesLoadedMsg struct {
	queues    []sqs.Queue
	nextToken *string
	err       error
}

// queueAttributesMsg is sent when a queue's attributes are fetched.
type queueAttributesMsg struct {
	attrs *sqs.QueueAttributes
	url   string // URL to match against current cursor
	err   error
}

// messagesLoadedMsg is sent when peek messages completes.
type messagesLoadedMsg struct {
	messages []sqs.Message
	err      error
}
