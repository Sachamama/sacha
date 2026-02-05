package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// mockDynamoDBAPI is a mock implementation of DynamoDBAPI for testing.
type mockDynamoDBAPI struct {
	listTablesFn    func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	describeTableFn func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	scanFn          func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

func (m *mockDynamoDBAPI) ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	if m.listTablesFn != nil {
		return m.listTablesFn(ctx, params, optFns...)
	}
	return &dynamodb.ListTablesOutput{}, nil
}

func (m *mockDynamoDBAPI) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if m.describeTableFn != nil {
		return m.describeTableFn(ctx, params, optFns...)
	}
	return &dynamodb.DescribeTableOutput{}, nil
}

func (m *mockDynamoDBAPI) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if m.scanFn != nil {
		return m.scanFn(ctx, params, optFns...)
	}
	return &dynamodb.ScanOutput{}, nil
}

func TestListTables(t *testing.T) {
	tests := []struct {
		name       string
		mockFn     func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
		wantTables []Table
		wantToken  *string
		wantErr    bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				return &dynamodb.ListTablesOutput{
					TableNames: []string{},
				}, nil
			},
			wantTables: []Table{},
			wantErr:    false,
		},
		{
			name: "single table",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				return &dynamodb.ListTablesOutput{
					TableNames: []string{"users"},
				}, nil
			},
			wantTables: []Table{{Name: "users"}},
			wantErr:    false,
		},
		{
			name: "multiple tables",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				return &dynamodb.ListTablesOutput{
					TableNames: []string{"users", "orders", "products"},
				}, nil
			},
			wantTables: []Table{
				{Name: "users"},
				{Name: "orders"},
				{Name: "products"},
			},
			wantErr: false,
		},
		{
			name: "with pagination token",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				return &dynamodb.ListTablesOutput{
					TableNames:             []string{"table1"},
					LastEvaluatedTableName: aws.String("table1"),
				}, nil
			},
			wantTables: []Table{{Name: "table1"}},
			wantToken:  aws.String("table1"),
			wantErr:    false,
		},
		{
			name: "uses exclusive start table name",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				if params.ExclusiveStartTableName == nil || *params.ExclusiveStartTableName != "start-here" {
					t.Errorf("expected ExclusiveStartTableName 'start-here', got %v", params.ExclusiveStartTableName)
				}
				return &dynamodb.ListTablesOutput{
					TableNames: []string{"table2"},
				}, nil
			},
			wantTables: []Table{{Name: "table2"}},
			wantErr:    false,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockDynamoDBAPI{listTablesFn: tt.mockFn},
			}

			var startToken *string
			if tt.name == "uses exclusive start table name" {
				startToken = aws.String("start-here")
			}

			tables, token, err := client.ListTables(context.Background(), startToken)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListTables() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(tables) != len(tt.wantTables) {
				t.Fatalf("got %d tables, want %d", len(tables), len(tt.wantTables))
			}

			for i, table := range tables {
				if table.Name != tt.wantTables[i].Name {
					t.Errorf("table[%d].Name = %q, want %q", i, table.Name, tt.wantTables[i].Name)
				}
			}

			if (token == nil) != (tt.wantToken == nil) {
				t.Errorf("token = %v, wantToken = %v", token, tt.wantToken)
			}
		})
	}
}

func TestDescribeTable(t *testing.T) {
	tests := []struct {
		name    string
		table   string
		mockFn  func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
		want    *TableDescription
		wantErr bool
	}{
		{
			name:  "basic table",
			table: "users",
			mockFn: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
				if *params.TableName != "users" {
					t.Errorf("unexpected table name: %s", *params.TableName)
				}
				return &dynamodb.DescribeTableOutput{
					Table: &types.TableDescription{
						TableName:   aws.String("users"),
						TableStatus: types.TableStatusActive,
						ItemCount:   aws.Int64(1000),
						TableSizeBytes: aws.Int64(524288),
						KeySchema: []types.KeySchemaElement{
							{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
						},
						AttributeDefinitions: []types.AttributeDefinition{
							{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
						},
					},
				}, nil
			},
			want: &TableDescription{
				Name:           "users",
				Status:         "ACTIVE",
				ItemCount:      1000,
				TableSizeBytes: 524288,
				BillingMode:    "PAY_PER_REQUEST",
				KeySchema: []KeySchema{
					{AttributeName: "id", KeyType: "HASH"},
				},
				AttributeDefinitions: []AttributeDefinition{
					{Name: "id", Type: "S"},
				},
			},
			wantErr: false,
		},
		{
			name:  "table with GSI",
			table: "orders",
			mockFn: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
				return &dynamodb.DescribeTableOutput{
					Table: &types.TableDescription{
						TableName:   aws.String("orders"),
						TableStatus: types.TableStatusActive,
						ItemCount:   aws.Int64(500),
						KeySchema: []types.KeySchemaElement{
							{AttributeName: aws.String("orderId"), KeyType: types.KeyTypeHash},
						},
						AttributeDefinitions: []types.AttributeDefinition{
							{AttributeName: aws.String("orderId"), AttributeType: types.ScalarAttributeTypeS},
							{AttributeName: aws.String("userId"), AttributeType: types.ScalarAttributeTypeS},
						},
						GlobalSecondaryIndexes: []types.GlobalSecondaryIndexDescription{
							{
								IndexName:   aws.String("userId-index"),
								IndexStatus: types.IndexStatusActive,
								KeySchema: []types.KeySchemaElement{
									{AttributeName: aws.String("userId"), KeyType: types.KeyTypeHash},
								},
							},
						},
					},
				}, nil
			},
			want: &TableDescription{
				Name:        "orders",
				Status:      "ACTIVE",
				ItemCount:   500,
				BillingMode: "PAY_PER_REQUEST",
				KeySchema: []KeySchema{
					{AttributeName: "orderId", KeyType: "HASH"},
				},
				AttributeDefinitions: []AttributeDefinition{
					{Name: "orderId", Type: "S"},
					{Name: "userId", Type: "S"},
				},
				GlobalSecondaryIndexes: []IndexDescription{
					{
						Name:   "userId-index",
						Status: "ACTIVE",
						KeySchema: []KeySchema{
							{AttributeName: "userId", KeyType: "HASH"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:  "provisioned table",
			table: "products",
			mockFn: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
				return &dynamodb.DescribeTableOutput{
					Table: &types.TableDescription{
						TableName:   aws.String("products"),
						TableStatus: types.TableStatusActive,
						ItemCount:   aws.Int64(200),
						KeySchema: []types.KeySchemaElement{
							{AttributeName: aws.String("sku"), KeyType: types.KeyTypeHash},
						},
						AttributeDefinitions: []types.AttributeDefinition{
							{AttributeName: aws.String("sku"), AttributeType: types.ScalarAttributeTypeS},
						},
						ProvisionedThroughput: &types.ProvisionedThroughputDescription{
							ReadCapacityUnits:  aws.Int64(10),
							WriteCapacityUnits: aws.Int64(5),
						},
					},
				}, nil
			},
			want: &TableDescription{
				Name:          "products",
				Status:        "ACTIVE",
				ItemCount:     200,
				BillingMode:   "PROVISIONED",
				ReadCapacity:  10,
				WriteCapacity: 5,
				KeySchema: []KeySchema{
					{AttributeName: "sku", KeyType: "HASH"},
				},
				AttributeDefinitions: []AttributeDefinition{
					{Name: "sku", Type: "S"},
				},
			},
			wantErr: false,
		},
		{
			name:  "API error",
			table: "nonexistent",
			mockFn: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
				return nil, errors.New("table not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockDynamoDBAPI{describeTableFn: tt.mockFn},
			}

			desc, err := client.DescribeTable(context.Background(), tt.table)

			if (err != nil) != tt.wantErr {
				t.Fatalf("DescribeTable() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if desc.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", desc.Name, tt.want.Name)
			}
			if desc.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", desc.Status, tt.want.Status)
			}
			if desc.ItemCount != tt.want.ItemCount {
				t.Errorf("ItemCount = %d, want %d", desc.ItemCount, tt.want.ItemCount)
			}
			if desc.BillingMode != tt.want.BillingMode {
				t.Errorf("BillingMode = %q, want %q", desc.BillingMode, tt.want.BillingMode)
			}
			if len(desc.KeySchema) != len(tt.want.KeySchema) {
				t.Fatalf("KeySchema length = %d, want %d", len(desc.KeySchema), len(tt.want.KeySchema))
			}
			for i, ks := range desc.KeySchema {
				if ks.AttributeName != tt.want.KeySchema[i].AttributeName {
					t.Errorf("KeySchema[%d].AttributeName = %q, want %q", i, ks.AttributeName, tt.want.KeySchema[i].AttributeName)
				}
				if ks.KeyType != tt.want.KeySchema[i].KeyType {
					t.Errorf("KeySchema[%d].KeyType = %q, want %q", i, ks.KeyType, tt.want.KeySchema[i].KeyType)
				}
			}
			if len(desc.GlobalSecondaryIndexes) != len(tt.want.GlobalSecondaryIndexes) {
				t.Fatalf("GSI length = %d, want %d", len(desc.GlobalSecondaryIndexes), len(tt.want.GlobalSecondaryIndexes))
			}
			for i, gsi := range desc.GlobalSecondaryIndexes {
				if gsi.Name != tt.want.GlobalSecondaryIndexes[i].Name {
					t.Errorf("GSI[%d].Name = %q, want %q", i, gsi.Name, tt.want.GlobalSecondaryIndexes[i].Name)
				}
			}
		})
	}
}

func TestScan(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		mockFn    func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
		wantItems int
		wantMore  bool
		wantErr   bool
	}{
		{
			name:  "empty table",
			table: "empty",
			mockFn: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{},
				}, nil
			},
			wantItems: 0,
			wantErr:   false,
		},
		{
			name:  "items with string and number attributes",
			table: "users",
			mockFn: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{
							"id":   &types.AttributeValueMemberS{Value: "user-1"},
							"name": &types.AttributeValueMemberS{Value: "Alice"},
							"age":  &types.AttributeValueMemberN{Value: "30"},
						},
						{
							"id":   &types.AttributeValueMemberS{Value: "user-2"},
							"name": &types.AttributeValueMemberS{Value: "Bob"},
							"age":  &types.AttributeValueMemberN{Value: "25"},
						},
					},
					ScannedCount: 2,
				}, nil
			},
			wantItems: 2,
			wantErr:   false,
		},
		{
			name:  "with pagination",
			table: "large",
			mockFn: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{"id": &types.AttributeValueMemberS{Value: "1"}},
					},
					LastEvaluatedKey: map[string]types.AttributeValue{
						"id": &types.AttributeValueMemberS{Value: "1"},
					},
					ScannedCount: 1,
				}, nil
			},
			wantItems: 1,
			wantMore:  true,
			wantErr:   false,
		},
		{
			name:  "API error",
			table: "bad",
			mockFn: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api: &mockDynamoDBAPI{scanFn: tt.mockFn},
			}

			result, err := client.Scan(context.Background(), tt.table, nil, 25)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Scan() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(result.Items) != tt.wantItems {
				t.Errorf("got %d items, want %d", len(result.Items), tt.wantItems)
			}

			hasMore := result.LastEvaluatedKey != nil
			if hasMore != tt.wantMore {
				t.Errorf("hasMore = %v, want %v", hasMore, tt.wantMore)
			}
		})
	}
}

func TestFormatAttributeValue(t *testing.T) {
	tests := []struct {
		name string
		av   types.AttributeValue
		want string
	}{
		{
			name: "string",
			av:   &types.AttributeValueMemberS{Value: "hello"},
			want: "hello",
		},
		{
			name: "number",
			av:   &types.AttributeValueMemberN{Value: "42"},
			want: "42",
		},
		{
			name: "boolean true",
			av:   &types.AttributeValueMemberBOOL{Value: true},
			want: "true",
		},
		{
			name: "boolean false",
			av:   &types.AttributeValueMemberBOOL{Value: false},
			want: "false",
		},
		{
			name: "null",
			av:   &types.AttributeValueMemberNULL{Value: true},
			want: "NULL",
		},
		{
			name: "binary",
			av:   &types.AttributeValueMemberB{Value: []byte("data")},
			want: "<binary 4 bytes>",
		},
		{
			name: "string set",
			av:   &types.AttributeValueMemberSS{Value: []string{"a", "b", "c"}},
			want: "[a, b, c]",
		},
		{
			name: "number set",
			av:   &types.AttributeValueMemberNS{Value: []string{"1", "2", "3"}},
			want: "[1, 2, 3]",
		},
		{
			name: "list",
			av: &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberS{Value: "x"},
				&types.AttributeValueMemberN{Value: "1"},
			}},
			want: "[x, 1]",
		},
		{
			name: "map",
			av: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"key": &types.AttributeValueMemberS{Value: "val"},
			}},
			want: "{key: val}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttributeValue(tt.av)
			if got != tt.want {
				t.Errorf("formatAttributeValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestItemKeys(t *testing.T) {
	item := Item{
		"zebra": "1",
		"alpha": "2",
		"middle": "3",
	}

	keys := ItemKeys(item)

	if len(keys) != 3 {
		t.Fatalf("got %d keys, want 3", len(keys))
	}

	expected := []string{"alpha", "middle", "zebra"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestRawLastEvaluatedKey(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := RawLastEvaluatedKey(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("with attribute values", func(t *testing.T) {
		av := &types.AttributeValueMemberS{Value: "test"}
		input := map[string]interface{}{
			"id": av,
		}

		result := RawLastEvaluatedKey(input)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if _, ok := result["id"]; !ok {
			t.Error("expected 'id' key in result")
		}
	})

	t.Run("skips non-attribute values", func(t *testing.T) {
		input := map[string]interface{}{
			"bad": "not an attribute value",
		}

		result := RawLastEvaluatedKey(input)
		if _, ok := result["bad"]; ok {
			t.Error("expected 'bad' key to be skipped")
		}
	})
}
