package sqs

import "time"

// Queue represents an SQS queue.
type Queue struct {
	Name string
	URL  string
}

// QueueAttributes contains detailed attributes for an SQS queue.
type QueueAttributes struct {
	ARN                    string
	ApproximateMessages    int64
	ApproximateNotVisible  int64
	ApproximateDelayed     int64
	CreatedTimestamp       time.Time
	LastModifiedTimestamp  time.Time
	VisibilityTimeout      string
	MaxMessageSize         string
	MessageRetentionPeriod string
	DelaySeconds           string
	IsFIFO                 bool
	ContentDeduplication   bool
	RedrivePolicy          string
	EncryptionType         string
}

// Message represents an SQS message from a peek operation.
type Message struct {
	ID            string
	Body          string
	MD5           string
	SentTimestamp time.Time
	Attributes    map[string]string
}
