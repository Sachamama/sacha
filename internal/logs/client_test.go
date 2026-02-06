package logs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// mockCloudWatchLogsAPI is a mock implementation of CloudWatchLogsAPI for testing.
type mockCloudWatchLogsAPI struct {
	describeLogGroupsFn     func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	filterLogEventsFn       func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
	createLogGroupFn        func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	deleteLogGroupFn        func(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	putRetentionPolicyFn    func(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	deleteRetentionPolicyFn func(ctx context.Context, params *cloudwatchlogs.DeleteRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteRetentionPolicyOutput, error)
}

func (m *mockCloudWatchLogsAPI) DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if m.describeLogGroupsFn != nil {
		return m.describeLogGroupsFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (m *mockCloudWatchLogsAPI) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	if m.filterLogEventsFn != nil {
		return m.filterLogEventsFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.FilterLogEventsOutput{}, nil
}

func (m *mockCloudWatchLogsAPI) CreateLogGroup(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if m.createLogGroupFn != nil {
		return m.createLogGroupFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.CreateLogGroupOutput{}, nil
}

func (m *mockCloudWatchLogsAPI) DeleteLogGroup(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
	if m.deleteLogGroupFn != nil {
		return m.deleteLogGroupFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.DeleteLogGroupOutput{}, nil
}

func (m *mockCloudWatchLogsAPI) PutRetentionPolicy(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if m.putRetentionPolicyFn != nil {
		return m.putRetentionPolicyFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
}

func (m *mockCloudWatchLogsAPI) DeleteRetentionPolicy(ctx context.Context, params *cloudwatchlogs.DeleteRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteRetentionPolicyOutput, error) {
	if m.deleteRetentionPolicyFn != nil {
		return m.deleteRetentionPolicyFn(ctx, params, optFns...)
	}
	return &cloudwatchlogs.DeleteRetentionPolicyOutput{}, nil
}

func TestListLogGroups(t *testing.T) {
	tests := []struct {
		name       string
		mockFn     func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
		nextToken  *string
		wantGroups []LogGroup
		wantToken  *string
		wantErr    bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []types.LogGroup{},
				}, nil
			},
			wantGroups: []LogGroup{},
			wantToken:  nil,
			wantErr:    false,
		},
		{
			name: "single log group",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []types.LogGroup{
						{
							LogGroupName:    aws.String("/aws/lambda/my-function"),
							RetentionInDays: aws.Int32(30),
							StoredBytes:     aws.Int64(1024),
						},
					},
				}, nil
			},
			wantGroups: []LogGroup{
				{
					Name:          "/aws/lambda/my-function",
					RetentionDays: 30,
					StoredBytes:   1024,
				},
			},
			wantToken: nil,
			wantErr:   false,
		},
		{
			name: "multiple log groups with pagination",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []types.LogGroup{
						{LogGroupName: aws.String("/aws/lambda/func1")},
						{LogGroupName: aws.String("/aws/lambda/func2")},
					},
					NextToken: aws.String("next-page-token"),
				}, nil
			},
			wantGroups: []LogGroup{
				{Name: "/aws/lambda/func1"},
				{Name: "/aws/lambda/func2"},
			},
			wantToken: aws.String("next-page-token"),
			wantErr:   false,
		},
		{
			name: "passes next token to API",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				if params.NextToken == nil || *params.NextToken != "input-token" {
					t.Errorf("expected NextToken to be 'input-token', got %v", params.NextToken)
				}
				return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
			},
			nextToken:  aws.String("input-token"),
			wantGroups: []LogGroup{},
			wantErr:    false,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockCloudWatchLogsAPI{describeLogGroupsFn: tt.mockFn},
			}

			groups, token, err := client.ListLogGroups(context.Background(), tt.nextToken)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListLogGroups() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(groups) != len(tt.wantGroups) {
				t.Fatalf("got %d groups, want %d", len(groups), len(tt.wantGroups))
			}

			for i, g := range groups {
				if g.Name != tt.wantGroups[i].Name {
					t.Errorf("group[%d].Name = %q, want %q", i, g.Name, tt.wantGroups[i].Name)
				}
				if g.RetentionDays != tt.wantGroups[i].RetentionDays {
					t.Errorf("group[%d].RetentionDays = %d, want %d", i, g.RetentionDays, tt.wantGroups[i].RetentionDays)
				}
				if g.StoredBytes != tt.wantGroups[i].StoredBytes {
					t.Errorf("group[%d].StoredBytes = %d, want %d", i, g.StoredBytes, tt.wantGroups[i].StoredBytes)
				}
			}

			if (token == nil) != (tt.wantToken == nil) {
				t.Errorf("token = %v, wantToken = %v", token, tt.wantToken)
			}
			if token != nil && tt.wantToken != nil && *token != *tt.wantToken {
				t.Errorf("token = %q, want %q", *token, *tt.wantToken)
			}
		})
	}
}

func TestCreateLogGroup(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		mockFn    func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
		wantErr   bool
	}{
		{
			name:      "successful creation",
			groupName: "/aws/lambda/new-function",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
				if params.LogGroupName == nil || *params.LogGroupName != "/aws/lambda/new-function" {
					t.Errorf("expected LogGroupName to be '/aws/lambda/new-function', got %v", params.LogGroupName)
				}
				return &cloudwatchlogs.CreateLogGroupOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:      "API error - already exists",
			groupName: "/aws/lambda/existing",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
				return nil, errors.New("ResourceAlreadyExistsException")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockCloudWatchLogsAPI{createLogGroupFn: tt.mockFn},
			}

			err := client.CreateLogGroup(context.Background(), tt.groupName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateLogGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFetchEvents(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		groups     []string
		startTime  time.Time
		mockFn     func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
		wantEvents int
		wantErr    bool
	}{
		{
			name:      "empty groups",
			groups:    []string{},
			startTime: baseTime,
			mockFn: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				t.Error("should not call API for empty groups")
				return nil, nil
			},
			wantEvents: 0,
			wantErr:    false,
		},
		{
			name:      "single group with events",
			groups:    []string{"/aws/lambda/func1"},
			startTime: baseTime,
			mockFn: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				if *params.LogGroupName != "/aws/lambda/func1" {
					t.Errorf("unexpected log group: %s", *params.LogGroupName)
				}
				if *params.StartTime != baseTime.UnixMilli() {
					t.Errorf("unexpected start time: %d", *params.StartTime)
				}
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []types.FilteredLogEvent{
						{
							Timestamp:     aws.Int64(baseTime.Add(1 * time.Second).UnixMilli()),
							LogStreamName: aws.String("stream1"),
							Message:       aws.String("log message 1"),
						},
						{
							Timestamp:     aws.Int64(baseTime.Add(2 * time.Second).UnixMilli()),
							LogStreamName: aws.String("stream1"),
							Message:       aws.String("log message 2"),
						},
					},
				}, nil
			},
			wantEvents: 2,
			wantErr:    false,
		},
		{
			name:      "multiple groups aggregated and sorted",
			groups:    []string{"/aws/lambda/func1", "/aws/lambda/func2"},
			startTime: baseTime,
			mockFn: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				switch *params.LogGroupName {
				case "/aws/lambda/func1":
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []types.FilteredLogEvent{
							{
								Timestamp:     aws.Int64(baseTime.Add(3 * time.Second).UnixMilli()),
								LogStreamName: aws.String("stream1"),
								Message:       aws.String("func1 message"),
							},
						},
					}, nil
				case "/aws/lambda/func2":
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []types.FilteredLogEvent{
							{
								Timestamp:     aws.Int64(baseTime.Add(1 * time.Second).UnixMilli()),
								LogStreamName: aws.String("stream2"),
								Message:       aws.String("func2 message"),
							},
						},
					}, nil
				default:
					t.Errorf("unexpected log group: %s", *params.LogGroupName)
					return nil, nil
				}
			},
			wantEvents: 2,
			wantErr:    false,
		},
		{
			name:      "API error",
			groups:    []string{"/aws/lambda/func1"},
			startTime: baseTime,
			mockFn: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return nil, errors.New("throttling exception")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockCloudWatchLogsAPI{filterLogEventsFn: tt.mockFn},
			}

			events, nextStart, err := client.FetchEvents(context.Background(), tt.groups, tt.startTime)

			if (err != nil) != tt.wantErr {
				t.Fatalf("FetchEvents() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(events) != tt.wantEvents {
				t.Errorf("got %d events, want %d", len(events), tt.wantEvents)
			}

			// Verify events are sorted by timestamp
			for i := 1; i < len(events); i++ {
				if events[i].Timestamp.Before(events[i-1].Timestamp) {
					t.Errorf("events not sorted: event[%d].Timestamp (%v) < event[%d].Timestamp (%v)",
						i, events[i].Timestamp, i-1, events[i-1].Timestamp)
				}
			}

			// Verify nextStart is after the last event (or unchanged if no events)
			if len(events) > 0 {
				lastEventTime := events[len(events)-1].Timestamp
				if !nextStart.After(lastEventTime) {
					t.Errorf("nextStart (%v) should be after last event (%v)", nextStart, lastEventTime)
				}
			}
		})
	}
}

func TestFetchEventsLogGroupAssignment(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	client := &Client{
		api: &mockCloudWatchLogsAPI{
			filterLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []types.FilteredLogEvent{
						{
							Timestamp:     aws.Int64(baseTime.UnixMilli()),
							LogStreamName: aws.String("stream"),
							Message:       aws.String("message"),
						},
					},
				}, nil
			},
		},
	}

	events, _, err := client.FetchEvents(context.Background(), []string{"/my/log/group"}, baseTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].LogGroup != "/my/log/group" {
		t.Errorf("LogGroup = %q, want %q", events[0].LogGroup, "/my/log/group")
	}
}

func TestDeleteLogGroup(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		mockFn    func(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
		wantErr   bool
	}{
		{
			name:      "successful deletion",
			groupName: "/aws/lambda/my-function",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
				if params.LogGroupName == nil || *params.LogGroupName != "/aws/lambda/my-function" {
					t.Errorf("expected LogGroupName to be '/aws/lambda/my-function', got %v", params.LogGroupName)
				}
				return &cloudwatchlogs.DeleteLogGroupOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:      "API error - not found",
			groupName: "/aws/lambda/missing",
			mockFn: func(ctx context.Context, params *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
				return nil, errors.New("ResourceNotFoundException")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockCloudWatchLogsAPI{deleteLogGroupFn: tt.mockFn},
			}

			err := client.DeleteLogGroup(context.Background(), tt.groupName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("DeleteLogGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetRetentionPolicy(t *testing.T) {
	tests := []struct {
		name          string
		groupName     string
		retentionDays int32
		wantPut       bool // expect PutRetentionPolicy call
		wantDelete    bool // expect DeleteRetentionPolicy call
		putErr        error
		deleteErr     error
		wantErr       bool
	}{
		{
			name:          "set 30 day retention",
			groupName:     "/aws/lambda/func1",
			retentionDays: 30,
			wantPut:       true,
			wantErr:       false,
		},
		{
			name:          "remove retention (never expire)",
			groupName:     "/aws/lambda/func1",
			retentionDays: 0,
			wantDelete:    true,
			wantErr:       false,
		},
		{
			name:          "put retention API error",
			groupName:     "/aws/lambda/func1",
			retentionDays: 7,
			wantPut:       true,
			putErr:        errors.New("access denied"),
			wantErr:       true,
		},
		{
			name:          "delete retention API error",
			groupName:     "/aws/lambda/func1",
			retentionDays: 0,
			wantDelete:    true,
			deleteErr:     errors.New("access denied"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			putCalled := false
			deleteCalled := false

			client := &Client{
				api: &mockCloudWatchLogsAPI{
					putRetentionPolicyFn: func(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
						putCalled = true
						if *params.LogGroupName != tt.groupName {
							t.Errorf("LogGroupName = %q, want %q", *params.LogGroupName, tt.groupName)
						}
						if *params.RetentionInDays != tt.retentionDays {
							t.Errorf("RetentionInDays = %d, want %d", *params.RetentionInDays, tt.retentionDays)
						}
						if tt.putErr != nil {
							return nil, tt.putErr
						}
						return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
					},
					deleteRetentionPolicyFn: func(ctx context.Context, params *cloudwatchlogs.DeleteRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteRetentionPolicyOutput, error) {
						deleteCalled = true
						if *params.LogGroupName != tt.groupName {
							t.Errorf("LogGroupName = %q, want %q", *params.LogGroupName, tt.groupName)
						}
						if tt.deleteErr != nil {
							return nil, tt.deleteErr
						}
						return &cloudwatchlogs.DeleteRetentionPolicyOutput{}, nil
					},
				},
			}

			err := client.SetRetentionPolicy(context.Background(), tt.groupName, tt.retentionDays)

			if (err != nil) != tt.wantErr {
				t.Fatalf("SetRetentionPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantPut && !putCalled {
				t.Error("expected PutRetentionPolicy to be called")
			}
			if tt.wantDelete && !deleteCalled {
				t.Error("expected DeleteRetentionPolicy to be called")
			}
			if !tt.wantPut && putCalled {
				t.Error("unexpected PutRetentionPolicy call")
			}
			if !tt.wantDelete && deleteCalled {
				t.Error("unexpected DeleteRetentionPolicy call")
			}
		})
	}
}

func TestListLogGroupsCreationTime(t *testing.T) {
	creationMs := int64(1705312800000) // 2024-01-15T10:00:00Z

	client := &Client{
		api: &mockCloudWatchLogsAPI{
			describeLogGroupsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []types.LogGroup{
						{
							LogGroupName: aws.String("/test/group"),
							CreationTime: &creationMs,
						},
					},
				}, nil
			},
		},
	}

	groups, _, err := client.ListLogGroups(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if groups[0].CreationTime.IsZero() {
		t.Error("CreationTime should not be zero")
	}

	expected := time.Unix(0, creationMs*int64(time.Millisecond))
	if !groups[0].CreationTime.Equal(expected) {
		t.Errorf("CreationTime = %v, want %v", groups[0].CreationTime, expected)
	}
}
