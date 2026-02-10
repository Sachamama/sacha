package ec2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2API captures the AWS SDK methods we use.
type EC2API interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Client wraps the EC2 API for instance operations.
type Client struct {
	api EC2API
}

// NewClient creates a new EC2 client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: ec2.NewFromConfig(cfg),
	}
}

// ListInstances returns EC2 instances, paginated via token.
func (c *Client) ListInstances(ctx context.Context, nextToken *string) ([]Instance, *string, error) {
	input := &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(50),
	}
	if nextToken != nil {
		input.NextToken = nextToken
	}

	out, err := c.api.DescribeInstances(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("describe instances: %w", err)
	}

	var instances []Instance
	for _, reservation := range out.Reservations {
		for _, inst := range reservation.Instances {
			instances = append(instances, mapInstance(inst))
		}
	}

	return instances, out.NextToken, nil
}

func mapInstance(inst types.Instance) Instance {
	i := Instance{
		InstanceID:   aws.ToString(inst.InstanceId),
		InstanceType: string(inst.InstanceType),
		PrivateIP:    aws.ToString(inst.PrivateIpAddress),
		PublicIP:     aws.ToString(inst.PublicIpAddress),
		VpcID:        aws.ToString(inst.VpcId),
		SubnetID:     aws.ToString(inst.SubnetId),
		Architecture: string(inst.Architecture),
		ImageID:      aws.ToString(inst.ImageId),
		KeyName:      aws.ToString(inst.KeyName),
		Tags:         make(map[string]string),
	}

	if inst.State != nil {
		i.State = string(inst.State.Name)
	}

	if inst.Placement != nil {
		i.AvailabilityZone = aws.ToString(inst.Placement.AvailabilityZone)
	}

	if inst.PlatformDetails != nil {
		i.Platform = *inst.PlatformDetails
	}

	if inst.LaunchTime != nil {
		i.LaunchTime = *inst.LaunchTime
	}

	if inst.IamInstanceProfile != nil {
		i.IAMProfile = aws.ToString(inst.IamInstanceProfile.Arn)
	}

	for _, sg := range inst.SecurityGroups {
		i.SecurityGroups = append(i.SecurityGroups, SecurityGroup{
			ID:   aws.ToString(sg.GroupId),
			Name: aws.ToString(sg.GroupName),
		})
	}

	for _, tag := range inst.Tags {
		key := aws.ToString(tag.Key)
		val := aws.ToString(tag.Value)
		i.Tags[key] = val
		if key == "Name" {
			i.Name = val
		}
	}

	// Normalise empty time to zero value.
	if i.LaunchTime == (time.Time{}) {
		i.LaunchTime = time.Time{}
	}

	return i
}
