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
		groups = append(groups, LogGroup{
			Name:          aws.ToString(g.LogGroupName),
			RetentionDays: aws.ToInt32(g.RetentionInDays),
			StoredBytes:   aws.ToInt64(g.StoredBytes),
		})
	}

	return groups, out.NextToken, nil
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
