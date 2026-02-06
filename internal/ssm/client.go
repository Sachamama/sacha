package ssm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SSMAPI captures the AWS SDK methods we use.
type SSMAPI interface {
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

const pageSize = 50

// Client wraps the SSM API for parameter operations.
type Client struct {
	api SSMAPI
}

// NewClient creates a new SSM client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: ssm.NewFromConfig(cfg),
	}
}

// GetParametersByPath lists parameters under a given path prefix.
// It groups child prefixes as virtual folders and returns leaf parameters.
// Uses "/" delimiter logic: for path "/app/", returns immediate children only.
func (c *Client) GetParametersByPath(ctx context.Context, path string, token *string) ([]Parameter, *string, error) {
	if path == "" {
		path = "/"
	}

	out, err := c.api.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:           aws.String(path),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(false),
		MaxResults:     aws.Int32(pageSize),
		NextToken:      token,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("get parameters by path: %w", err)
	}

	// Group results into immediate children vs deeper paths
	prefixes := make(map[string]bool)
	var params []Parameter

	for _, p := range out.Parameters {
		name := aws.ToString(p.Name)
		// Strip the leading path to get the relative part
		relative := strings.TrimPrefix(name, path)
		if relative == "" {
			continue
		}

		// Check if this is a direct child or nested deeper
		if idx := strings.Index(relative, "/"); idx >= 0 {
			// Nested: extract the next path segment as a prefix
			prefix := path + relative[:idx+1]
			prefixes[prefix] = true
		} else {
			// Direct child parameter
			params = append(params, Parameter{
				Name:             name,
				Value:            aws.ToString(p.Value),
				Type:             string(p.Type),
				Version:          p.Version,
				LastModifiedDate: aws.ToTime(p.LastModifiedDate),
				ARN:              aws.ToString(p.ARN),
				DataType:         aws.ToString(p.DataType),
			})
		}
	}

	// Build result: prefixes first (sorted), then parameters (sorted)
	result := make([]Parameter, 0, len(prefixes)+len(params))

	sortedPrefixes := make([]string, 0, len(prefixes))
	for p := range prefixes {
		sortedPrefixes = append(sortedPrefixes, p)
	}
	sort.Strings(sortedPrefixes)

	for _, p := range sortedPrefixes {
		// Extract display name from prefix
		trimmed := strings.TrimSuffix(strings.TrimPrefix(p, path), "/")
		result = append(result, Parameter{
			Name:     p,
			IsPrefix: true,
			Value:    trimmed, // use Value field to store display name for prefixes
		})
	}

	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
	result = append(result, params...)

	var nextToken *string
	if out.NextToken != nil {
		nextToken = out.NextToken
	}

	return result, nextToken, nil
}

// GetParameter fetches a single parameter by name, with decryption.
func (c *Client) GetParameter(ctx context.Context, name string) (*Parameter, error) {
	out, err := c.api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get parameter %q: %w", name, err)
	}

	p := out.Parameter
	return &Parameter{
		Name:             aws.ToString(p.Name),
		Value:            aws.ToString(p.Value),
		Type:             string(p.Type),
		Version:          p.Version,
		LastModifiedDate: aws.ToTime(p.LastModifiedDate),
		ARN:              aws.ToString(p.ARN),
		DataType:         aws.ToString(p.DataType),
	}, nil
}

// ListTopLevelPaths returns the top-level path prefixes by describing all parameters
// and extracting unique first-level segments.
func (c *Client) ListTopLevelPaths(ctx context.Context, token *string) ([]Parameter, *string, error) {
	out, err := c.api.DescribeParameters(ctx, &ssm.DescribeParametersInput{
		MaxResults: aws.Int32(pageSize),
		NextToken:  token,
		ParameterFilters: []types.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Option: aws.String("BeginsWith"),
				Values: []string{"/"},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describe parameters: %w", err)
	}

	// Extract unique top-level prefixes
	prefixes := make(map[string]bool)
	var rootParams []Parameter

	for _, p := range out.Parameters {
		name := aws.ToString(p.Name)
		if !strings.HasPrefix(name, "/") {
			continue
		}
		// Strip leading "/" and find first segment
		rest := name[1:]
		idx := strings.Index(rest, "/")
		if idx >= 0 {
			prefix := "/" + rest[:idx+1]
			prefixes[prefix] = true
		} else {
			// Root-level parameter (e.g. "/my-param")
			rootParams = append(rootParams, Parameter{
				Name:             name,
				Type:             string(p.Type),
				Version:          p.Version,
				LastModifiedDate: aws.ToTime(p.LastModifiedDate),
			})
		}
	}

	result := make([]Parameter, 0, len(prefixes)+len(rootParams))

	sortedPrefixes := make([]string, 0, len(prefixes))
	for p := range prefixes {
		sortedPrefixes = append(sortedPrefixes, p)
	}
	sort.Strings(sortedPrefixes)

	for _, p := range sortedPrefixes {
		trimmed := strings.TrimPrefix(strings.TrimSuffix(p, "/"), "/")
		result = append(result, Parameter{
			Name:     p,
			IsPrefix: true,
			Value:    trimmed,
		})
	}

	sort.Slice(rootParams, func(i, j int) bool {
		return rootParams[i].Name < rootParams[j].Name
	})
	result = append(result, rootParams...)

	var nextToken *string
	if out.NextToken != nil {
		nextToken = out.NextToken
	}

	return result, nextToken, nil
}
