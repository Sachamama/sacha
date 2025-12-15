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
	load loadConfigFunc
}

// NewLoader returns a Loader that uses the default AWS SDK behavior.
func NewLoader() Loader {
	return Loader{
		load: config.LoadDefaultConfig,
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

	cfg, err := l.load(ctx, optFns...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	return cfg, nil
}
