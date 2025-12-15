package logs

import (
	"context"
	"fmt"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/logs"

	tea "github.com/charmbracelet/bubbletea"
)

// CloudWatchLogsService wires the CloudWatch Logs UI to the service registry.
type CloudWatchLogsService struct {
}

func (CloudWatchLogsService) Name() string {
	return "cloudwatch-logs"
}

func (CloudWatchLogsService) Title() string {
	return "CloudWatch Logs"
}

func (CloudWatchLogsService) Init(ctx context.Context, cfg sdkaws.Config, opts awsx.ServiceOptions) (tea.Model, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("region must be set before loading CloudWatch Logs")
	}
	client := logs.NewClient(cfg)
	model := NewModel(client)
	return model, nil
}
