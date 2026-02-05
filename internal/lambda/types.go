package lambda

import "time"

// Function represents a Lambda function summary.
type Function struct {
	Name    string
	Runtime string
	Handler string
	Memory  int32
	Timeout int32
}

// FunctionDetails contains detailed metadata about a Lambda function.
type FunctionDetails struct {
	Name             string
	Runtime          string
	Handler          string
	Description      string
	Memory           int32
	Timeout          int32
	CodeSize         int64
	LastModified     time.Time
	State            string
	Role             string
	ARN              string
	Layers           []string
	Architectures    []string
	PackageType      string
	EphemeralStorage int32
	Environment      map[string]string
}
