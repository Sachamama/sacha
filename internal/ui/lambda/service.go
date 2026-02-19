package lambda

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/lambda"
)

// LambdaService wires the Lambda browser UI to the service registry.
type LambdaService struct{}

// Name returns the service identifier.
func (LambdaService) Name() string {
	return "lambda"
}

// Title returns the display name for the service.
func (LambdaService) Title() string {
	return "Lambda"
}

// Init initializes the Lambda browser model.
func (LambdaService) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := lambda.NewClient(cfg)
	cacheKey := cache.Key{
		AccountID: opts.AccountID,
		Region:    cfg.Region,
		Service:   "lambda",
	}
	model := NewModel(client, opts.Cache, cacheKey)
	return model, nil
}
