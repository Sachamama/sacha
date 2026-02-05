package lambda

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// LambdaAPI captures the AWS SDK methods we use.
type LambdaAPI interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
	GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
}

// Client wraps the Lambda API for function operations.
type Client struct {
	api LambdaAPI
}

// NewClient creates a new Lambda client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: lambda.NewFromConfig(cfg),
	}
}

// ListFunctions returns Lambda functions, paginated via marker.
func (c *Client) ListFunctions(ctx context.Context, marker *string) ([]Function, *string, error) {
	input := &lambda.ListFunctionsInput{
		MaxItems: aws.Int32(50),
	}
	if marker != nil {
		input.Marker = marker
	}

	out, err := c.api.ListFunctions(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("list functions: %w", err)
	}

	functions := make([]Function, 0, len(out.Functions))
	for _, f := range out.Functions {
		fn := Function{
			Name:    aws.ToString(f.FunctionName),
			Runtime: string(f.Runtime),
			Handler: aws.ToString(f.Handler),
		}
		if f.MemorySize != nil {
			fn.Memory = *f.MemorySize
		}
		if f.Timeout != nil {
			fn.Timeout = *f.Timeout
		}
		functions = append(functions, fn)
	}

	return functions, out.NextMarker, nil
}

// GetFunction returns detailed information about a function.
func (c *Client) GetFunction(ctx context.Context, functionName string) (*FunctionDetails, error) {
	out, err := c.api.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return nil, fmt.Errorf("get function: %w", err)
	}

	cfg := out.Configuration
	details := &FunctionDetails{
		Name:        aws.ToString(cfg.FunctionName),
		Runtime:     string(cfg.Runtime),
		Handler:     aws.ToString(cfg.Handler),
		Description: aws.ToString(cfg.Description),
		CodeSize:    cfg.CodeSize,
		State:       string(cfg.State),
		Role:        aws.ToString(cfg.Role),
		ARN:         aws.ToString(cfg.FunctionArn),
		PackageType: string(cfg.PackageType),
	}

	if cfg.MemorySize != nil {
		details.Memory = *cfg.MemorySize
	}
	if cfg.Timeout != nil {
		details.Timeout = *cfg.Timeout
	}
	if cfg.EphemeralStorage != nil && cfg.EphemeralStorage.Size != nil {
		details.EphemeralStorage = *cfg.EphemeralStorage.Size
	}

	if cfg.LastModified != nil {
		if t, err := time.Parse("2006-01-02T15:04:05.000+0000", *cfg.LastModified); err == nil {
			details.LastModified = t
		}
	}

	for _, layer := range cfg.Layers {
		details.Layers = append(details.Layers, aws.ToString(layer.Arn))
	}

	for _, arch := range cfg.Architectures {
		details.Architectures = append(details.Architectures, string(arch))
	}

	if cfg.Environment != nil && cfg.Environment.Variables != nil {
		details.Environment = cfg.Environment.Variables
	}

	return details, nil
}
