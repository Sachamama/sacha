package ssm

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/ssm"
)

// SSMService wires the SSM Parameter Store browser UI to the service registry.
type SSMService struct{}

// Name returns the service identifier.
func (SSMService) Name() string {
	return "ssm"
}

// Title returns the display name for the service.
func (SSMService) Title() string {
	return "SSM Parameter Store"
}

// Init initializes the SSM browser model.
func (SSMService) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := ssm.NewClient(cfg)
	cacheKey := cache.Key{
		AccountID: opts.AccountID,
		Region:    cfg.Region,
		Service:   "ssm",
	}
	model := NewModel(client, opts.Cache, cacheKey)
	return model, nil
}
