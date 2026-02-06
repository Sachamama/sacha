package ssm

import "time"

// Parameter represents an SSM parameter.
type Parameter struct {
	Name             string
	Value            string
	Type             string // String, StringList, SecureString
	Version          int64
	LastModifiedDate time.Time
	ARN              string
	DataType         string
	IsPrefix         bool // true for path prefixes (virtual folders)
}

// DisplayName returns the last segment of the parameter name for display.
func (p Parameter) DisplayName() string {
	if p.Name == "" {
		return ""
	}
	for i := len(p.Name) - 1; i >= 0; i-- {
		if p.Name[i] == '/' {
			return p.Name[i+1:]
		}
	}
	return p.Name
}
