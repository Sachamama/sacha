package logs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// CloudWatchLogsAPI captures the AWS SDK methods we use.
type CloudWatchLogsAPI interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
	CreateLogGroup(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	DeleteLogGroup(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	PutRetentionPolicy(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	DeleteRetentionPolicy(ctx context.Context, params *cloudwatchlogs.DeleteRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteRetentionPolicyOutput, error)
}

type Client struct {
	api CloudWatchLogsAPI
}

func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: cloudwatchlogs.NewFromConfig(cfg),
	}
}

type LogGroup struct {
	Name          string
	Arn           string // full ARN, populated when cross-account is active
	AccountID     string // source account ID, extracted from ARN
	RetentionDays int32
	StoredBytes   int64
	CreationTime  time.Time
}

// ListLogGroupsOpts controls cross-account log group listing.
type ListLogGroupsOpts struct {
	IncludeLinkedAccounts bool
	AccountIdentifiers    []string
}

type TailEvent struct {
	Timestamp time.Time
	LogGroup  string
	LogStream string
	Message   string
}

// ListLogGroups returns a page of log groups and the next token, if any.
func (c *Client) ListLogGroups(ctx context.Context, nextToken *string) ([]LogGroup, *string, error) {
	return c.ListLogGroupsWithOpts(ctx, nextToken, ListLogGroupsOpts{})
}

// ListLogGroupsWithOpts returns a page of log groups with cross-account options.
func (c *Client) ListLogGroupsWithOpts(ctx context.Context, nextToken *string, opts ListLogGroupsOpts) ([]LogGroup, *string, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		NextToken: nextToken,
		Limit:     aws.Int32(50),
	}
	if opts.IncludeLinkedAccounts {
		input.IncludeLinkedAccounts = aws.Bool(true)
	}
	if len(opts.AccountIdentifiers) > 0 {
		input.AccountIdentifiers = opts.AccountIdentifiers
	}

	out, err := c.api.DescribeLogGroups(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("describe log groups: %w", err)
	}

	groups := make([]LogGroup, 0, len(out.LogGroups))
	for _, g := range out.LogGroups {
		var created time.Time
		if g.CreationTime != nil {
			created = time.Unix(0, *g.CreationTime*int64(time.Millisecond))
		}
		arn := aws.ToString(g.Arn)
		groups = append(groups, LogGroup{
			Name:          aws.ToString(g.LogGroupName),
			Arn:           arn,
			AccountID:     extractAccountFromARN(arn),
			RetentionDays: aws.ToInt32(g.RetentionInDays),
			StoredBytes:   aws.ToInt64(g.StoredBytes),
			CreationTime:  created,
		})
	}

	return groups, out.NextToken, nil
}

// CreateLogGroup creates a new log group with the given name.
func (c *Client) CreateLogGroup(ctx context.Context, name string) error {
	_, err := c.api.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("create log group: %w", err)
	}
	return nil
}

// FetchEvents pulls events from the provided log groups and returns them ordered by timestamp.
func (c *Client) FetchEvents(ctx context.Context, groups []string, start time.Time) ([]TailEvent, time.Time, error) {
	events := make([]TailEvent, 0)
	nextStart := start

	for _, group := range groups {
		input := &cloudwatchlogs.FilterLogEventsInput{
			StartTime: aws.Int64(start.UnixMilli()),
			Limit:     aws.Int32(100),
		}
		// Use LogGroupIdentifier for ARNs (cross-account), LogGroupName for names.
		if isARN(group) {
			input.LogGroupIdentifier = aws.String(group)
		} else {
			input.LogGroupName = aws.String(group)
		}
		out, err := c.api.FilterLogEvents(ctx, input)
		if err != nil {
			return nil, start, fmt.Errorf("filter log events: %w", err)
		}

		for _, e := range out.Events {
			ts := time.Unix(0, aws.ToInt64(e.Timestamp)*int64(time.Millisecond))
			if ts.After(nextStart) {
				nextStart = ts
			}
			events = append(events, TailEvent{
				Timestamp: ts,
				LogGroup:  group,
				LogStream: aws.ToString(e.LogStreamName),
				Message:   aws.ToString(e.Message),
			})
		}
	}

	sortEvents(events)
	if len(events) > 0 {
		// Resume from just after the last seen event.
		nextStart = events[len(events)-1].Timestamp.Add(time.Millisecond)
	}
	return events, nextStart, nil
}

// DeleteLogGroup deletes a log group by name.
func (c *Client) DeleteLogGroup(ctx context.Context, name string) error {
	_, err := c.api.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("delete log group: %w", err)
	}
	return nil
}

// SetRetentionPolicy sets the retention policy on a log group.
// Use retentionDays=0 to remove the retention policy (never expire).
func (c *Client) SetRetentionPolicy(ctx context.Context, name string, retentionDays int32) error {
	if retentionDays == 0 {
		_, err := c.api.DeleteRetentionPolicy(ctx, &cloudwatchlogs.DeleteRetentionPolicyInput{
			LogGroupName: aws.String(name),
		})
		if err != nil {
			return fmt.Errorf("delete retention policy: %w", err)
		}
		return nil
	}
	_, err := c.api.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(name),
		RetentionInDays: aws.Int32(retentionDays),
	})
	if err != nil {
		return fmt.Errorf("put retention policy: %w", err)
	}
	return nil
}

// isARN returns true if the string looks like an AWS ARN.
func isARN(s string) bool {
	return strings.HasPrefix(s, "arn:")
}

// extractAccountFromARN extracts the account ID from an ARN.
// Format: arn:aws:logs:REGION:ACCOUNT_ID:log-group:NAME
func extractAccountFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}
