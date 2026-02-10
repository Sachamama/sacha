package ec2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2API is a mock implementation of EC2API for testing.
type mockEC2API struct {
	describeInstancesFn func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2API) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.describeInstancesFn != nil {
		return m.describeInstancesFn(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func TestListInstances(t *testing.T) {
	tests := []struct {
		name          string
		mockFn        func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
		wantInstances int
		wantToken     *string
		wantErr       bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{},
				}, nil
			},
			wantInstances: 0,
			wantErr:       false,
		},
		{
			name: "single instance",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId:       aws.String("i-1234567890abcdef0"),
									InstanceType:     types.InstanceTypeT2Micro,
									PrivateIpAddress: aws.String("10.0.0.1"),
									PublicIpAddress:  aws.String("54.1.2.3"),
									State: &types.InstanceState{
										Name: types.InstanceStateNameRunning,
									},
									Tags: []types.Tag{
										{Key: aws.String("Name"), Value: aws.String("web-server")},
									},
								},
							},
						},
					},
				}, nil
			},
			wantInstances: 1,
			wantErr:       false,
		},
		{
			name: "multiple reservations with instances",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId:   aws.String("i-aaa"),
									InstanceType: types.InstanceTypeT2Micro,
									State:        &types.InstanceState{Name: types.InstanceStateNameRunning},
								},
								{
									InstanceId:   aws.String("i-bbb"),
									InstanceType: types.InstanceTypeT3Medium,
									State:        &types.InstanceState{Name: types.InstanceStateNameStopped},
								},
							},
						},
						{
							Instances: []types.Instance{
								{
									InstanceId:   aws.String("i-ccc"),
									InstanceType: types.InstanceTypeM5Large,
									State:        &types.InstanceState{Name: types.InstanceStateNameTerminated},
								},
							},
						},
					},
				}, nil
			},
			wantInstances: 3,
			wantErr:       false,
		},
		{
			name: "with pagination token",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{InstanceId: aws.String("i-page1"), State: &types.InstanceState{Name: types.InstanceStateNameRunning}},
							},
						},
					},
					NextToken: aws.String("next-page"),
				}, nil
			},
			wantInstances: 1,
			wantToken:     aws.String("next-page"),
			wantErr:       false,
		},
		{
			name: "uses token for continuation",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				if params.NextToken == nil || *params.NextToken != "start-here" {
					t.Errorf("expected NextToken 'start-here', got %v", params.NextToken)
				}
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{InstanceId: aws.String("i-page2"), State: &types.InstanceState{Name: types.InstanceStateNameRunning}},
							},
						},
					},
				}, nil
			},
			wantInstances: 1,
			wantErr:       false,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockEC2API{describeInstancesFn: tt.mockFn},
			}

			var startToken *string
			if tt.name == "uses token for continuation" {
				startToken = aws.String("start-here")
			}

			instances, token, err := client.ListInstances(context.Background(), startToken)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListInstances() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(instances) != tt.wantInstances {
				t.Fatalf("got %d instances, want %d", len(instances), tt.wantInstances)
			}

			if (token == nil) != (tt.wantToken == nil) {
				t.Errorf("token = %v, wantToken = %v", token, tt.wantToken)
			}
		})
	}
}

func TestMapInstance(t *testing.T) {
	t.Run("full instance fields", func(t *testing.T) {
		launchTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		inst := types.Instance{
			InstanceId:       aws.String("i-abc123"),
			InstanceType:     types.InstanceTypeT3Medium,
			PrivateIpAddress: aws.String("10.0.1.50"),
			PublicIpAddress:  aws.String("3.14.15.92"),
			State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
			VpcId:            aws.String("vpc-aaa"),
			SubnetId:         aws.String("subnet-bbb"),
			Architecture:     types.ArchitectureValuesX8664,
			PlatformDetails:  aws.String("Linux/UNIX"),
			ImageId:          aws.String("ami-12345"),
			KeyName:          aws.String("my-key"),
			LaunchTime:       &launchTime,
			Placement:        &types.Placement{AvailabilityZone: aws.String("us-east-1a")},
			IamInstanceProfile: &types.IamInstanceProfile{
				Arn: aws.String("arn:aws:iam::123456789012:instance-profile/my-profile"),
			},
			SecurityGroups: []types.GroupIdentifier{
				{GroupId: aws.String("sg-111"), GroupName: aws.String("my-sg")},
			},
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("web-server")},
				{Key: aws.String("env"), Value: aws.String("production")},
			},
		}

		result := mapInstance(inst)

		if result.InstanceID != "i-abc123" {
			t.Errorf("InstanceID = %q, want %q", result.InstanceID, "i-abc123")
		}
		if result.Name != "web-server" {
			t.Errorf("Name = %q, want %q", result.Name, "web-server")
		}
		if result.State != "running" {
			t.Errorf("State = %q, want %q", result.State, "running")
		}
		if result.InstanceType != "t3.medium" {
			t.Errorf("InstanceType = %q, want %q", result.InstanceType, "t3.medium")
		}
		if result.PrivateIP != "10.0.1.50" {
			t.Errorf("PrivateIP = %q, want %q", result.PrivateIP, "10.0.1.50")
		}
		if result.PublicIP != "3.14.15.92" {
			t.Errorf("PublicIP = %q, want %q", result.PublicIP, "3.14.15.92")
		}
		if result.VpcID != "vpc-aaa" {
			t.Errorf("VpcID = %q, want %q", result.VpcID, "vpc-aaa")
		}
		if result.SubnetID != "subnet-bbb" {
			t.Errorf("SubnetID = %q, want %q", result.SubnetID, "subnet-bbb")
		}
		if result.AvailabilityZone != "us-east-1a" {
			t.Errorf("AvailabilityZone = %q, want %q", result.AvailabilityZone, "us-east-1a")
		}
		if result.Architecture != "x86_64" {
			t.Errorf("Architecture = %q, want %q", result.Architecture, "x86_64")
		}
		if result.Platform != "Linux/UNIX" {
			t.Errorf("Platform = %q, want %q", result.Platform, "Linux/UNIX")
		}
		if result.ImageID != "ami-12345" {
			t.Errorf("ImageID = %q, want %q", result.ImageID, "ami-12345")
		}
		if result.KeyName != "my-key" {
			t.Errorf("KeyName = %q, want %q", result.KeyName, "my-key")
		}
		if !result.LaunchTime.Equal(launchTime) {
			t.Errorf("LaunchTime = %v, want %v", result.LaunchTime, launchTime)
		}
		if result.IAMProfile != "arn:aws:iam::123456789012:instance-profile/my-profile" {
			t.Errorf("IAMProfile = %q, want %q", result.IAMProfile, "arn:aws:iam::123456789012:instance-profile/my-profile")
		}
		if len(result.SecurityGroups) != 1 {
			t.Fatalf("SecurityGroups length = %d, want 1", len(result.SecurityGroups))
		}
		if result.SecurityGroups[0].ID != "sg-111" {
			t.Errorf("SecurityGroups[0].ID = %q, want %q", result.SecurityGroups[0].ID, "sg-111")
		}
		if result.SecurityGroups[0].Name != "my-sg" {
			t.Errorf("SecurityGroups[0].Name = %q, want %q", result.SecurityGroups[0].Name, "my-sg")
		}
		if result.Tags["env"] != "production" {
			t.Errorf("Tags[env] = %q, want %q", result.Tags["env"], "production")
		}
	})

	t.Run("minimal instance", func(t *testing.T) {
		inst := types.Instance{
			InstanceId: aws.String("i-minimal"),
			State:      &types.InstanceState{Name: types.InstanceStateNamePending},
		}

		result := mapInstance(inst)

		if result.InstanceID != "i-minimal" {
			t.Errorf("InstanceID = %q, want %q", result.InstanceID, "i-minimal")
		}
		if result.State != "pending" {
			t.Errorf("State = %q, want %q", result.State, "pending")
		}
		if result.Name != "" {
			t.Errorf("Name = %q, want empty", result.Name)
		}
		if result.PrivateIP != "" {
			t.Errorf("PrivateIP = %q, want empty", result.PrivateIP)
		}
	})
}
