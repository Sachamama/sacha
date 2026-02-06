package sqs

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// mockSQSAPI is a mock implementation of SQSAPI for testing.
type mockSQSAPI struct {
	listQueuesFn         func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
	getQueueAttributesFn func(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	receiveMessageFn     func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

func (m *mockSQSAPI) ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	if m.listQueuesFn != nil {
		return m.listQueuesFn(ctx, params, optFns...)
	}
	return &sqs.ListQueuesOutput{}, nil
}

func (m *mockSQSAPI) GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	if m.getQueueAttributesFn != nil {
		return m.getQueueAttributesFn(ctx, params, optFns...)
	}
	return &sqs.GetQueueAttributesOutput{}, nil
}

func (m *mockSQSAPI) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	if m.receiveMessageFn != nil {
		return m.receiveMessageFn(ctx, params, optFns...)
	}
	return &sqs.ReceiveMessageOutput{}, nil
}

func TestListQueues(t *testing.T) {
	tests := []struct {
		name      string
		token     *string
		mockFn    func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
		wantCount int
		wantToken bool
		wantErr   bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				return &sqs.ListQueuesOutput{}, nil
			},
			wantCount: 0,
		},
		{
			name: "returns queues with names extracted",
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				return &sqs.ListQueuesOutput{
					QueueUrls: []string{
						"https://sqs.us-east-1.amazonaws.com/123456789/my-queue",
						"https://sqs.us-east-1.amazonaws.com/123456789/other-queue.fifo",
					},
				}, nil
			},
			wantCount: 2,
		},
		{
			name: "pagination with token",
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				return &sqs.ListQueuesOutput{
					QueueUrls: []string{
						"https://sqs.us-east-1.amazonaws.com/123456789/queue1",
					},
					NextToken: aws.String("next-page"),
				}, nil
			},
			wantCount: 1,
			wantToken: true,
		},
		{
			name:  "passes continuation token",
			token: aws.String("continue-here"),
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				if params.NextToken == nil || *params.NextToken != "continue-here" {
					t.Errorf("expected token 'continue-here', got %v", params.NextToken)
				}
				return &sqs.ListQueuesOutput{}, nil
			},
			wantCount: 0,
		},
		{
			name: "uses page size",
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				if params.MaxResults == nil || *params.MaxResults != pageSize {
					t.Errorf("expected MaxResults %d, got %v", pageSize, params.MaxResults)
				}
				return &sqs.ListQueuesOutput{}, nil
			},
			wantCount: 0,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSQSAPI{listQueuesFn: tt.mockFn}}

			queues, nextToken, err := client.ListQueues(context.Background(), tt.token)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListQueues() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(queues) != tt.wantCount {
				t.Fatalf("got %d queues, want %d", len(queues), tt.wantCount)
			}

			if (nextToken != nil) != tt.wantToken {
				t.Errorf("nextToken = %v, wantToken = %v", nextToken, tt.wantToken)
			}
		})
	}
}

func TestListQueues_ExtractsNames(t *testing.T) {
	client := &Client{api: &mockSQSAPI{
		listQueuesFn: func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{
				QueueUrls: []string{
					"https://sqs.us-east-1.amazonaws.com/123456789/my-queue",
					"https://sqs.us-east-1.amazonaws.com/123456789/orders.fifo",
				},
			}, nil
		},
	}}

	queues, _, err := client.ListQueues(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if queues[0].Name != "my-queue" {
		t.Errorf("queue[0].Name = %q, want %q", queues[0].Name, "my-queue")
	}
	if queues[0].URL != "https://sqs.us-east-1.amazonaws.com/123456789/my-queue" {
		t.Errorf("queue[0].URL = %q, unexpected", queues[0].URL)
	}
	if queues[1].Name != "orders.fifo" {
		t.Errorf("queue[1].Name = %q, want %q", queues[1].Name, "orders.fifo")
	}
}

func TestGetQueueAttributes(t *testing.T) {
	tests := []struct {
		name    string
		mockFn  func(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
		wantErr bool
	}{
		{
			name: "returns all attributes",
			mockFn: func(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
				if *params.QueueUrl != "https://sqs.us-east-1.amazonaws.com/123/test-queue" {
					t.Errorf("unexpected queue URL: %s", *params.QueueUrl)
				}
				// Verify we request All attributes
				found := false
				for _, attr := range params.AttributeNames {
					if attr == types.QueueAttributeNameAll {
						found = true
					}
				}
				if !found {
					t.Error("expected QueueAttributeNameAll in request")
				}
				return &sqs.GetQueueAttributesOutput{
					Attributes: map[string]string{
						"QueueArn":                              "arn:aws:sqs:us-east-1:123:test-queue",
						"ApproximateNumberOfMessages":           "42",
						"ApproximateNumberOfMessagesNotVisible": "5",
						"ApproximateNumberOfMessagesDelayed":    "3",
						"CreatedTimestamp":                      "1700000000",
						"LastModifiedTimestamp":                 "1700001000",
						"VisibilityTimeout":                     "30",
						"MaximumMessageSize":                    "262144",
						"MessageRetentionPeriod":                "345600",
						"DelaySeconds":                          "0",
						"FifoQueue":                             "true",
						"ContentBasedDeduplication":             "true",
						"RedrivePolicy":                         `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123:dlq","maxReceiveCount":3}`,
						"SqsManagedSseEnabled":                  "true",
					},
				}, nil
			},
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
				return nil, errors.New("queue not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSQSAPI{getQueueAttributesFn: tt.mockFn}}

			attrs, err := client.GetQueueAttributes(context.Background(), "https://sqs.us-east-1.amazonaws.com/123/test-queue")

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetQueueAttributes() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if attrs.ARN != "arn:aws:sqs:us-east-1:123:test-queue" {
				t.Errorf("ARN = %q", attrs.ARN)
			}
			if attrs.ApproximateMessages != 42 {
				t.Errorf("ApproximateMessages = %d, want 42", attrs.ApproximateMessages)
			}
			if attrs.ApproximateNotVisible != 5 {
				t.Errorf("ApproximateNotVisible = %d, want 5", attrs.ApproximateNotVisible)
			}
			if attrs.ApproximateDelayed != 3 {
				t.Errorf("ApproximateDelayed = %d, want 3", attrs.ApproximateDelayed)
			}
			if !attrs.IsFIFO {
				t.Error("expected IsFIFO = true")
			}
			if !attrs.ContentDeduplication {
				t.Error("expected ContentDeduplication = true")
			}
			if attrs.VisibilityTimeout != "30" {
				t.Errorf("VisibilityTimeout = %q, want '30'", attrs.VisibilityTimeout)
			}
			if attrs.RedrivePolicy == "" {
				t.Error("expected RedrivePolicy to be set")
			}
		})
	}
}

func TestPeekMessages(t *testing.T) {
	tests := []struct {
		name      string
		mockFn    func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
		wantCount int
		wantErr   bool
	}{
		{
			name: "empty queue",
			mockFn: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
				return &sqs.ReceiveMessageOutput{}, nil
			},
			wantCount: 0,
		},
		{
			name: "returns messages with visibility timeout 0",
			mockFn: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
				if params.VisibilityTimeout != 0 {
					t.Errorf("expected VisibilityTimeout 0, got %d", params.VisibilityTimeout)
				}
				if params.MaxNumberOfMessages != 10 {
					t.Errorf("expected MaxNumberOfMessages 10, got %d", params.MaxNumberOfMessages)
				}
				return &sqs.ReceiveMessageOutput{
					Messages: []types.Message{
						{
							MessageId: aws.String("msg-1"),
							Body:      aws.String(`{"event":"order_created"}`),
							MD5OfBody: aws.String("abc123"),
							Attributes: map[string]string{
								"SentTimestamp":           "1700000000000",
								"ApproximateReceiveCount": "1",
							},
						},
						{
							MessageId: aws.String("msg-2"),
							Body:      aws.String("plain text message"),
							MD5OfBody: aws.String("def456"),
							Attributes: map[string]string{
								"SentTimestamp": "1700000001000",
							},
						},
					},
				}, nil
			},
			wantCount: 2,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSQSAPI{receiveMessageFn: tt.mockFn}}

			messages, err := client.PeekMessages(context.Background(), "https://sqs.us-east-1.amazonaws.com/123/test-queue", 10)

			if (err != nil) != tt.wantErr {
				t.Fatalf("PeekMessages() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(messages) != tt.wantCount {
				t.Fatalf("got %d messages, want %d", len(messages), tt.wantCount)
			}
		})
	}
}

func TestPeekMessages_ParsesFields(t *testing.T) {
	client := &Client{api: &mockSQSAPI{
		receiveMessageFn: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						MessageId: aws.String("msg-abc"),
						Body:      aws.String(`{"key":"value"}`),
						MD5OfBody: aws.String("md5hash"),
						Attributes: map[string]string{
							"SentTimestamp": "1700000000000",
						},
					},
				},
			}, nil
		},
	}}

	messages, err := client.PeekMessages(context.Background(), "https://sqs.us-east-1.amazonaws.com/123/q", 10)
	if err != nil {
		t.Fatal(err)
	}

	msg := messages[0]
	if msg.ID != "msg-abc" {
		t.Errorf("ID = %q, want 'msg-abc'", msg.ID)
	}
	if msg.Body != `{"key":"value"}` {
		t.Errorf("Body = %q", msg.Body)
	}
	if msg.MD5 != "md5hash" {
		t.Errorf("MD5 = %q", msg.MD5)
	}
	if msg.SentTimestamp.IsZero() {
		t.Error("SentTimestamp should not be zero")
	}
	if msg.Attributes["SentTimestamp"] != "1700000000000" {
		t.Errorf("Attributes[SentTimestamp] = %q", msg.Attributes["SentTimestamp"])
	}
}

func TestExtractQueueName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://sqs.us-east-1.amazonaws.com/123456789/my-queue", "my-queue"},
		{"https://sqs.us-east-1.amazonaws.com/123456789/orders.fifo", "orders.fifo"},
		{"my-queue", "my-queue"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractQueueName(tt.url)
			if got != tt.want {
				t.Errorf("extractQueueName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
