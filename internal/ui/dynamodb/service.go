package dynamodb

import (
	"context"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/dynamodb"
)

// DynamoDBService wires the DynamoDB browser UI to the service registry.
type DynamoDBService struct{}

// Name returns the service identifier.
func (DynamoDBService) Name() string {
	return "dynamodb"
}

// Title returns the display name for the service.
func (DynamoDBService) Title() string {
	return "DynamoDB"
}

// Init initializes the DynamoDB browser model.
func (DynamoDBService) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	client := dynamodb.NewClient(cfg)
	cacheKey := cache.Key{
		AccountID: opts.AccountID,
		Region:    cfg.Region,
		Service:   "dynamodb",
	}
	model := NewModel(client, opts.Cache, cacheKey)
	return model, nil
}
