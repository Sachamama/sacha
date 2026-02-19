package awsx

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type loadConfigFunc func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)

// Loader wraps AWS SDK configuration loading to allow injection in tests.
type Loader struct {
	load     loadConfigFunc
	endpoint string
}

// NewLoader returns a Loader that uses the default AWS SDK behavior.
// If endpoint is non-empty, it is applied as the base endpoint for all services.
func NewLoader(endpoint string) Loader {
	return Loader{
		load:     config.LoadDefaultConfig,
		endpoint: endpoint,
	}
}

// NewTestLoader creates a Loader with a custom load function for testing.
func NewTestLoader(fn loadConfigFunc, endpoint string) Loader {
	return Loader{
		load:     fn,
		endpoint: endpoint,
	}
}

// Load builds an aws.Config using optional profile and region overrides.
func (l Loader) Load(ctx context.Context, profile, region string) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{}
	if profile != "" {
		optFns = append(optFns, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		optFns = append(optFns, config.WithRegion(region))
	}
	if l.endpoint != "" {
		optFns = append(optFns, config.WithBaseEndpoint(l.endpoint))
	}

	cfg, err := l.load(ctx, optFns...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	return cfg, nil
}
