package sqs

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/sqs"
)

// SQSService wires the SQS Queue Browser UI to the service registry.
type SQSService struct{}

// Name returns the service identifier.
func (SQSService) Name() string {
	return "sqs"
}

// Title returns the display name for the service.
func (SQSService) Title() string {
	return "SQS Queues"
}

// Init initializes the SQS browser model.
func (SQSService) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := sqs.NewClient(cfg)
	model := NewModel(client)
	return model, nil
}
