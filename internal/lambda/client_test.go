package lambda

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// mockLambdaAPI is a mock implementation of LambdaAPI for testing.
type mockLambdaAPI struct {
	listFunctionsFn func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
	getFunctionFn   func(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
}

func (m *mockLambdaAPI) ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	if m.listFunctionsFn != nil {
		return m.listFunctionsFn(ctx, params, optFns...)
	}
	return &lambda.ListFunctionsOutput{}, nil
}

func (m *mockLambdaAPI) GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	if m.getFunctionFn != nil {
		return m.getFunctionFn(ctx, params, optFns...)
	}
	return &lambda.GetFunctionOutput{}, nil
}

func TestListFunctions(t *testing.T) {
	tests := []struct {
		name          string
		mockFn        func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
		wantFunctions []Function
		wantToken     *string
		wantErr       bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				return &lambda.ListFunctionsOutput{
					Functions: []types.FunctionConfiguration{},
				}, nil
			},
			wantFunctions: []Function{},
			wantErr:       false,
		},
		{
			name: "single function",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				return &lambda.ListFunctionsOutput{
					Functions: []types.FunctionConfiguration{
						{
							FunctionName: aws.String("my-func"),
							Runtime:      types.RuntimeNodejs20x,
							Handler:      aws.String("index.handler"),
							MemorySize:   aws.Int32(128),
							Timeout:      aws.Int32(30),
						},
					},
				}, nil
			},
			wantFunctions: []Function{
				{Name: "my-func", Runtime: "nodejs20.x", Handler: "index.handler", Memory: 128, Timeout: 30},
			},
			wantErr: false,
		},
		{
			name: "multiple functions",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				return &lambda.ListFunctionsOutput{
					Functions: []types.FunctionConfiguration{
						{FunctionName: aws.String("func-a"), Runtime: types.RuntimePython312},
						{FunctionName: aws.String("func-b"), Runtime: types.RuntimeGo1x},
						{FunctionName: aws.String("func-c"), Runtime: types.RuntimeJava21},
					},
				}, nil
			},
			wantFunctions: []Function{
				{Name: "func-a", Runtime: "python3.12"},
				{Name: "func-b", Runtime: "go1.x"},
				{Name: "func-c", Runtime: "java21"},
			},
			wantErr: false,
		},
		{
			name: "with pagination token",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				return &lambda.ListFunctionsOutput{
					Functions: []types.FunctionConfiguration{
						{FunctionName: aws.String("func-1")},
					},
					NextMarker: aws.String("next-page"),
				}, nil
			},
			wantFunctions: []Function{{Name: "func-1"}},
			wantToken:     aws.String("next-page"),
			wantErr:       false,
		},
		{
			name: "uses marker for continuation",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				if params.Marker == nil || *params.Marker != "start-here" {
					t.Errorf("expected Marker 'start-here', got %v", params.Marker)
				}
				return &lambda.ListFunctionsOutput{
					Functions: []types.FunctionConfiguration{
						{FunctionName: aws.String("func-2")},
					},
				}, nil
			},
			wantFunctions: []Function{{Name: "func-2"}},
			wantErr:       false,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockLambdaAPI{listFunctionsFn: tt.mockFn},
			}

			var startToken *string
			if tt.name == "uses marker for continuation" {
				startToken = aws.String("start-here")
			}

			functions, token, err := client.ListFunctions(context.Background(), startToken)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListFunctions() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(functions) != len(tt.wantFunctions) {
				t.Fatalf("got %d functions, want %d", len(functions), len(tt.wantFunctions))
			}

			for i, fn := range functions {
				if fn.Name != tt.wantFunctions[i].Name {
					t.Errorf("function[%d].Name = %q, want %q", i, fn.Name, tt.wantFunctions[i].Name)
				}
				if fn.Runtime != tt.wantFunctions[i].Runtime {
					t.Errorf("function[%d].Runtime = %q, want %q", i, fn.Runtime, tt.wantFunctions[i].Runtime)
				}
			}

			if (token == nil) != (tt.wantToken == nil) {
				t.Errorf("token = %v, wantToken = %v", token, tt.wantToken)
			}
		})
	}
}

func TestGetFunction(t *testing.T) {
	tests := []struct {
		name    string
		funcN   string
		mockFn  func(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
		want    *FunctionDetails
		wantErr bool
	}{
		{
			name:  "basic function",
			funcN: "my-func",
			mockFn: func(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
				if *params.FunctionName != "my-func" {
					t.Errorf("unexpected function name: %s", *params.FunctionName)
				}
				return &lambda.GetFunctionOutput{
					Configuration: &types.FunctionConfiguration{
						FunctionName: aws.String("my-func"),
						FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-func"),
						Runtime:      types.RuntimeNodejs20x,
						Handler:      aws.String("index.handler"),
						Description:  aws.String("My function"),
						MemorySize:   aws.Int32(256),
						Timeout:      aws.Int32(60),
						CodeSize:     1024,
						State:        types.StateActive,
						Role:         aws.String("arn:aws:iam::123456789012:role/my-role"),
						PackageType:  types.PackageTypeZip,
						Architectures: []types.Architecture{
							types.ArchitectureArm64,
						},
					},
				}, nil
			},
			want: &FunctionDetails{
				Name:          "my-func",
				ARN:           "arn:aws:lambda:us-east-1:123456789012:function:my-func",
				Runtime:       "nodejs20.x",
				Handler:       "index.handler",
				Description:   "My function",
				Memory:        256,
				Timeout:       60,
				CodeSize:      1024,
				State:         "Active",
				Role:          "arn:aws:iam::123456789012:role/my-role",
				PackageType:   "Zip",
				Architectures: []string{"arm64"},
			},
			wantErr: false,
		},
		{
			name:  "function with layers and env",
			funcN: "advanced-func",
			mockFn: func(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
				return &lambda.GetFunctionOutput{
					Configuration: &types.FunctionConfiguration{
						FunctionName: aws.String("advanced-func"),
						FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:advanced-func"),
						Runtime:      types.RuntimePython312,
						MemorySize:   aws.Int32(512),
						Timeout:      aws.Int32(300),
						State:        types.StateActive,
						Layers: []types.Layer{
							{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:layer:my-layer:1")},
						},
						Environment: &types.EnvironmentResponse{
							Variables: map[string]string{
								"ENV": "production",
							},
						},
						EphemeralStorage: &types.EphemeralStorage{
							Size: aws.Int32(1024),
						},
					},
				}, nil
			},
			want: &FunctionDetails{
				Name:             "advanced-func",
				ARN:              "arn:aws:lambda:us-east-1:123456789012:function:advanced-func",
				Runtime:          "python3.12",
				Memory:           512,
				Timeout:          300,
				State:            "Active",
				PackageType:      "Zip",
				EphemeralStorage: 1024,
				Layers:           []string{"arn:aws:lambda:us-east-1:123456789012:layer:my-layer:1"},
				Environment:      map[string]string{"ENV": "production"},
			},
			wantErr: false,
		},
		{
			name:  "API error",
			funcN: "nonexistent",
			mockFn: func(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
				return nil, errors.New("function not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockLambdaAPI{getFunctionFn: tt.mockFn},
			}

			details, err := client.GetFunction(context.Background(), tt.funcN)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetFunction() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if details.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", details.Name, tt.want.Name)
			}
			if details.ARN != tt.want.ARN {
				t.Errorf("ARN = %q, want %q", details.ARN, tt.want.ARN)
			}
			if details.Runtime != tt.want.Runtime {
				t.Errorf("Runtime = %q, want %q", details.Runtime, tt.want.Runtime)
			}
			if details.Memory != tt.want.Memory {
				t.Errorf("Memory = %d, want %d", details.Memory, tt.want.Memory)
			}
			if details.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %d, want %d", details.Timeout, tt.want.Timeout)
			}
			if details.State != tt.want.State {
				t.Errorf("State = %q, want %q", details.State, tt.want.State)
			}
			if len(details.Layers) != len(tt.want.Layers) {
				t.Fatalf("Layers length = %d, want %d", len(details.Layers), len(tt.want.Layers))
			}
			for i, l := range details.Layers {
				if l != tt.want.Layers[i] {
					t.Errorf("Layers[%d] = %q, want %q", i, l, tt.want.Layers[i])
				}
			}
			if len(details.Architectures) != len(tt.want.Architectures) {
				t.Fatalf("Architectures length = %d, want %d", len(details.Architectures), len(tt.want.Architectures))
			}
		})
	}
}
