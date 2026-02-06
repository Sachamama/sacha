package sqs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSAPI captures the AWS SDK methods we use.
type SQSAPI interface {
	ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

const pageSize = 50

// Client wraps the SQS API for queue operations.
type Client struct {
	api SQSAPI
}

// NewClient creates a new SQS client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: sqs.NewFromConfig(cfg),
	}
}

// ListQueues returns SQS queues with optional pagination.
func (c *Client) ListQueues(ctx context.Context, token *string) ([]Queue, *string, error) {
	out, err := c.api.ListQueues(ctx, &sqs.ListQueuesInput{
		MaxResults: aws.Int32(pageSize),
		NextToken:  token,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list queues: %w", err)
	}

	queues := make([]Queue, 0, len(out.QueueUrls))
	for _, url := range out.QueueUrls {
		queues = append(queues, Queue{
			Name: extractQueueName(url),
			URL:  url,
		})
	}

	var nextToken *string
	if out.NextToken != nil {
		nextToken = out.NextToken
	}

	return queues, nextToken, nil
}

// GetQueueAttributes fetches detailed attributes for a queue.
func (c *Client) GetQueueAttributes(ctx context.Context, queueURL string) (*QueueAttributes, error) {
	out, err := c.api.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get queue attributes: %w", err)
	}

	attrs := out.Attributes
	return &QueueAttributes{
		ARN:                    attrs["QueueArn"],
		ApproximateMessages:    parseInt64(attrs["ApproximateNumberOfMessages"]),
		ApproximateNotVisible:  parseInt64(attrs["ApproximateNumberOfMessagesNotVisible"]),
		ApproximateDelayed:     parseInt64(attrs["ApproximateNumberOfMessagesDelayed"]),
		CreatedTimestamp:       parseUnixTimestamp(attrs["CreatedTimestamp"]),
		LastModifiedTimestamp:  parseUnixTimestamp(attrs["LastModifiedTimestamp"]),
		VisibilityTimeout:      attrs["VisibilityTimeout"],
		MaxMessageSize:         attrs["MaximumMessageSize"],
		MessageRetentionPeriod: attrs["MessageRetentionPeriod"],
		DelaySeconds:           attrs["DelaySeconds"],
		IsFIFO:                 attrs["FifoQueue"] == "true",
		ContentDeduplication:   attrs["ContentBasedDeduplication"] == "true",
		RedrivePolicy:          attrs["RedrivePolicy"],
		EncryptionType:         attrs["SqsManagedSseEnabled"],
	}, nil
}

// PeekMessages receives messages without removing them from the queue.
// Uses VisibilityTimeout=0 so messages remain available to other consumers.
func (c *Client) PeekMessages(ctx context.Context, queueURL string, maxMessages int32) ([]Message, error) {
	out, err := c.api.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: maxMessages,
		VisibilityTimeout:   0, // Don't hide messages from other consumers
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("receive messages: %w", err)
	}

	messages := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msg := Message{
			ID:         aws.ToString(m.MessageId),
			Body:       aws.ToString(m.Body),
			MD5:        aws.ToString(m.MD5OfBody),
			Attributes: make(map[string]string),
		}
		for k, v := range m.Attributes {
			msg.Attributes[k] = v
		}
		if ts, ok := m.Attributes["SentTimestamp"]; ok {
			msg.SentTimestamp = parseMilliTimestamp(ts)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// extractQueueName extracts the queue name from its URL.
func extractQueueName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseUnixTimestamp(s string) time.Time {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(v, 0)
}

func parseMilliTimestamp(s string) time.Time {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(v)
}
