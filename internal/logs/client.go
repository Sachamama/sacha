package logs

import (
	"context"
	"fmt"
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
	RetentionDays int32
	StoredBytes   int64
	CreationTime  time.Time
}

type TailEvent struct {
	Timestamp time.Time
	LogGroup  string
	LogStream string
	Message   string
}

// ListLogGroups returns a page of log groups and the next token, if any.
func (c *Client) ListLogGroups(ctx context.Context, nextToken *string) ([]LogGroup, *string, error) {
	out, err := c.api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		NextToken: nextToken,
		Limit:     aws.Int32(50),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describe log groups: %w", err)
	}

	groups := make([]LogGroup, 0, len(out.LogGroups))
	for _, g := range out.LogGroups {
		var created time.Time
		if g.CreationTime != nil {
			created = time.Unix(0, *g.CreationTime*int64(time.Millisecond))
		}
		groups = append(groups, LogGroup{
			Name:          aws.ToString(g.LogGroupName),
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
		out, err := c.api.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName: aws.String(group),
			StartTime:    aws.Int64(start.UnixMilli()),
			Limit:        aws.Int32(100),
		})
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
