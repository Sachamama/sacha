package ssm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSMAPI is a mock implementation of SSMAPI for testing.
type mockSSMAPI struct {
	getParametersByPathFn func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	getParameterFn        func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	describeParametersFn  func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

func (m *mockSSMAPI) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	if m.getParametersByPathFn != nil {
		return m.getParametersByPathFn(ctx, params, optFns...)
	}
	return &ssm.GetParametersByPathOutput{}, nil
}

func (m *mockSSMAPI) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.getParameterFn != nil {
		return m.getParameterFn(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockSSMAPI) DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	if m.describeParametersFn != nil {
		return m.describeParametersFn(ctx, params, optFns...)
	}
	return &ssm.DescribeParametersOutput{}, nil
}

func TestGetParametersByPath(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		path       string
		token      *string
		mockFn     func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
		wantCount  int
		wantPrefix int // number of prefix entries expected
		wantToken  bool
		wantErr    bool
	}{
		{
			name: "empty response",
			path: "/app/",
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{}, nil
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "direct children only",
			path: "/app/",
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				if *params.Path != "/app/" {
					t.Errorf("expected path '/app/', got %q", *params.Path)
				}
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{
							Name:             aws.String("/app/db-host"),
							Value:            aws.String("localhost"),
							Type:             types.ParameterTypeString,
							Version:          1,
							LastModifiedDate: aws.Time(now),
							ARN:              aws.String("arn:aws:ssm:us-east-1:123:parameter/app/db-host"),
						},
						{
							Name:             aws.String("/app/db-port"),
							Value:            aws.String("5432"),
							Type:             types.ParameterTypeString,
							Version:          2,
							LastModifiedDate: aws.Time(now),
						},
					},
				}, nil
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "mixed folders and parameters",
			path: "/app/",
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/app/prod/db-host"), Value: aws.String("prod.db"), Type: types.ParameterTypeString},
						{Name: aws.String("/app/prod/db-port"), Value: aws.String("5432"), Type: types.ParameterTypeString},
						{Name: aws.String("/app/staging/db-host"), Value: aws.String("staging.db"), Type: types.ParameterTypeString},
						{Name: aws.String("/app/api-key"), Value: aws.String("secret"), Type: types.ParameterTypeSecureString},
					},
				}, nil
			},
			wantCount:  3, // 2 prefixes (prod/, staging/) + 1 direct param (api-key)
			wantPrefix: 2,
			wantErr:    false,
		},
		{
			name: "pagination with token",
			path: "/app/",
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/app/param1"), Value: aws.String("val1"), Type: types.ParameterTypeString},
					},
					NextToken: aws.String("next-page"),
				}, nil
			},
			wantCount: 1,
			wantToken: true,
			wantErr:   false,
		},
		{
			name:  "passes continuation token",
			path:  "/app/",
			token: aws.String("continue-here"),
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				if params.NextToken == nil || *params.NextToken != "continue-here" {
					t.Errorf("expected token 'continue-here', got %v", params.NextToken)
				}
				return &ssm.GetParametersByPathOutput{}, nil
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "API error",
			path: "/app/",
			mockFn: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSSMAPI{getParametersByPathFn: tt.mockFn}}

			params, nextToken, err := client.GetParametersByPath(context.Background(), tt.path, tt.token)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetParametersByPath() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(params) != tt.wantCount {
				t.Fatalf("got %d params, want %d", len(params), tt.wantCount)
			}

			if tt.wantPrefix > 0 {
				prefixCount := 0
				for _, p := range params {
					if p.IsPrefix {
						prefixCount++
					}
				}
				if prefixCount != tt.wantPrefix {
					t.Errorf("got %d prefixes, want %d", prefixCount, tt.wantPrefix)
				}
			}

			if (nextToken != nil) != tt.wantToken {
				t.Errorf("nextToken = %v, wantToken = %v", nextToken, tt.wantToken)
			}
		})
	}
}

func TestGetParameter(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		param   string
		mockFn  func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
		want    *Parameter
		wantErr bool
	}{
		{
			name:  "successful retrieval",
			param: "/app/db-host",
			mockFn: func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				if *params.Name != "/app/db-host" {
					t.Errorf("unexpected name: %s", *params.Name)
				}
				if !*params.WithDecryption {
					t.Error("expected WithDecryption to be true")
				}
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Name:             aws.String("/app/db-host"),
						Value:            aws.String("prod.database.internal"),
						Type:             types.ParameterTypeString,
						Version:          3,
						LastModifiedDate: aws.Time(now),
						ARN:              aws.String("arn:aws:ssm:us-east-1:123:parameter/app/db-host"),
						DataType:         aws.String("text"),
					},
				}, nil
			},
			want: &Parameter{
				Name:             "/app/db-host",
				Value:            "prod.database.internal",
				Type:             "String",
				Version:          3,
				LastModifiedDate: now,
				ARN:              "arn:aws:ssm:us-east-1:123:parameter/app/db-host",
				DataType:         "text",
			},
			wantErr: false,
		},
		{
			name:  "API error",
			param: "/nonexistent",
			mockFn: func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return nil, errors.New("parameter not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSSMAPI{getParameterFn: tt.mockFn}}

			param, err := client.GetParameter(context.Background(), tt.param)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetParameter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if param.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", param.Name, tt.want.Name)
			}
			if param.Value != tt.want.Value {
				t.Errorf("Value = %q, want %q", param.Value, tt.want.Value)
			}
			if param.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", param.Type, tt.want.Type)
			}
			if param.Version != tt.want.Version {
				t.Errorf("Version = %d, want %d", param.Version, tt.want.Version)
			}
			if param.ARN != tt.want.ARN {
				t.Errorf("ARN = %q, want %q", param.ARN, tt.want.ARN)
			}
		})
	}
}

func TestListTopLevelPaths(t *testing.T) {
	tests := []struct {
		name      string
		mockFn    func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
		wantCount int
		wantToken bool
		wantErr   bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return &ssm.DescribeParametersOutput{}, nil
			},
			wantCount: 0,
		},
		{
			name: "groups into top-level prefixes",
			mockFn: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return &ssm.DescribeParametersOutput{
					Parameters: []types.ParameterMetadata{
						{Name: aws.String("/app/prod/db-host"), Type: types.ParameterTypeString, Version: 1},
						{Name: aws.String("/app/staging/db-host"), Type: types.ParameterTypeString, Version: 1},
						{Name: aws.String("/infra/vpc-id"), Type: types.ParameterTypeString, Version: 1},
					},
				}, nil
			},
			wantCount: 2, // /app/ and /infra/
		},
		{
			name: "root-level parameters",
			mockFn: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return &ssm.DescribeParametersOutput{
					Parameters: []types.ParameterMetadata{
						{Name: aws.String("/my-param"), Type: types.ParameterTypeString, Version: 1},
						{Name: aws.String("/app/db-host"), Type: types.ParameterTypeString, Version: 1},
					},
				}, nil
			},
			wantCount: 2, // /app/ prefix + /my-param root param
		},
		{
			name: "pagination",
			mockFn: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return &ssm.DescribeParametersOutput{
					Parameters: []types.ParameterMetadata{
						{Name: aws.String("/app/key"), Type: types.ParameterTypeString, Version: 1},
					},
					NextToken: aws.String("page2"),
				}, nil
			},
			wantCount: 1,
			wantToken: true,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{api: &mockSSMAPI{describeParametersFn: tt.mockFn}}

			params, nextToken, err := client.ListTopLevelPaths(context.Background(), nil)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListTopLevelPaths() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(params) != tt.wantCount {
				t.Errorf("got %d params, want %d", len(params), tt.wantCount)
			}

			if (nextToken != nil) != tt.wantToken {
				t.Errorf("nextToken = %v, wantToken = %v", nextToken, tt.wantToken)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"/app/prod/db-host", "db-host"},
		{"/app/api-key", "api-key"},
		{"/my-param", "my-param"},
		{"no-slash", "no-slash"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parameter{Name: tt.name}
			got := p.DisplayName()
			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
