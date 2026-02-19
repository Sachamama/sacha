package ec2

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/ec2"
)

// EC2Service wires the EC2 browser UI to the service registry.
type EC2Service struct{}

// Name returns the service identifier.
func (EC2Service) Name() string {
	return "ec2"
}

// Title returns the display name for the service.
func (EC2Service) Title() string {
	return "EC2"
}

// Init initializes the EC2 browser model.
func (EC2Service) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := ec2.NewClient(cfg)
	cacheKey := cache.Key{
		AccountID: opts.AccountID,
		Region:    cfg.Region,
		Service:   "ec2",
	}
	model := NewModel(client, opts.Cache, cacheKey)
	return model, nil
}
