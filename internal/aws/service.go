package awsx

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
)

// Service defines a pluggable AWS-backed UI module.
type Service interface {
	Name() string
	Title() string
	Init(ctx context.Context, cfg aws.Config, opts ServiceOptions) (tea.Model, error)
}

// ServiceOptions contains dependencies shared with services.
type ServiceOptions struct {
	Logger ServiceLogger
}

// ServiceLogger is a narrow logging interface used by services.
type ServiceLogger interface {
	Debug(msg string, kv ...interface{})
	Info(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}
