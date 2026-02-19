package ec2

import "github.com/sachamama/sacha/internal/ec2"

// instancesLoadedMsg is sent when instance listing completes.
type instancesLoadedMsg struct {
	instances []ec2.Instance
	nextToken *string
	err       error
}

// moreInstancesLoadedMsg is sent when additional instances are loaded.
type moreInstancesLoadedMsg struct {
	instances []ec2.Instance
	nextToken *string
	err       error
}

