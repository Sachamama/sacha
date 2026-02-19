package s3

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/s3"
)

// S3Service wires the S3 browser UI to the service registry.
type S3Service struct{}

// Name returns the service identifier.
func (S3Service) Name() string {
	return "s3"
}

// Title returns the display name for the service.
func (S3Service) Title() string {
	return "S3"
}

// Init initializes the S3 browser model.
func (S3Service) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := s3.NewClient(cfg)
	cacheKey := cache.Key{
		AccountID: opts.AccountID,
		Region:    cfg.Region,
		Service:   "s3",
	}
	model := NewModel(client, opts.Cache, cacheKey)
	return model, nil
}
