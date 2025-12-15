package awsx

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func TestLoaderAppliesRegionAndProfile(t *testing.T) {
	var captured config.LoadOptions
	loader := Loader{
		load: func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			for _, fn := range optFns {
				if err := fn(&captured); err != nil {
					return aws.Config{}, err
				}
			}
			return aws.Config{}, nil
		},
	}

	if _, err := loader.Load(context.Background(), "profile-name", "us-east-1"); err != nil {
		t.Fatalf("load: %v", err)
	}

	if captured.SharedConfigProfile != "profile-name" {
		t.Fatalf("profile not applied, got %s", captured.SharedConfigProfile)
	}
	if captured.Region != "us-east-1" {
		t.Fatalf("region not applied, got %s", captured.Region)
	}
}
