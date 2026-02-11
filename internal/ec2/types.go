package ec2

import "time"

// Instance represents an EC2 instance.
type Instance struct {
	InstanceID       string
	Name             string // from Name tag
	State            string // running, stopped, terminated, etc.
	InstanceType     string
	PrivateIP        string
	PublicIP         string
	LaunchTime       time.Time
	VpcID            string
	SubnetID         string
	AvailabilityZone string
	Architecture     string
	Platform         string
	ImageID          string
	KeyName          string
	SecurityGroups   []SecurityGroup
	IAMProfile       string
	Tags             map[string]string
}

// SecurityGroup is a minimal reference to an EC2 security group.
type SecurityGroup struct {
	ID   string
	Name string
}
