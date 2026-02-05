package dynamodb

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBAPI captures the AWS SDK methods we use.
type DynamoDBAPI interface {
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

// Client wraps the DynamoDB API for table and item operations.
type Client struct {
	api DynamoDBAPI
}

// NewClient creates a new DynamoDB client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		api: dynamodb.NewFromConfig(cfg),
	}
}

// ListTables returns DynamoDB tables, paginated via exclusiveStartTableName.
func (c *Client) ListTables(ctx context.Context, exclusiveStartTableName *string) ([]Table, *string, error) {
	input := &dynamodb.ListTablesInput{
		Limit: aws.Int32(100),
	}
	if exclusiveStartTableName != nil {
		input.ExclusiveStartTableName = exclusiveStartTableName
	}

	out, err := c.api.ListTables(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("list tables: %w", err)
	}

	tables := make([]Table, 0, len(out.TableNames))
	for _, name := range out.TableNames {
		tables = append(tables, Table{Name: name})
	}

	return tables, out.LastEvaluatedTableName, nil
}

// DescribeTable returns detailed information about a table.
func (c *Client) DescribeTable(ctx context.Context, tableName string) (*TableDescription, error) {
	out, err := c.api.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe table: %w", err)
	}

	t := out.Table

	desc := &TableDescription{
		Name:             aws.ToString(t.TableName),
		Status:           string(t.TableStatus),
		ItemCount:        aws.ToInt64(t.ItemCount),
		TableSizeBytes:   aws.ToInt64(t.TableSizeBytes),
		CreationDateTime: aws.ToTime(t.CreationDateTime),
		BillingMode:      "PAY_PER_REQUEST",
	}

	if t.BillingModeSummary != nil {
		desc.BillingMode = string(t.BillingModeSummary.BillingMode)
	}

	if t.ProvisionedThroughput != nil {
		desc.ReadCapacity = aws.ToInt64(t.ProvisionedThroughput.ReadCapacityUnits)
		desc.WriteCapacity = aws.ToInt64(t.ProvisionedThroughput.WriteCapacityUnits)
		if desc.ReadCapacity > 0 || desc.WriteCapacity > 0 {
			desc.BillingMode = "PROVISIONED"
		}
	}

	for _, ks := range t.KeySchema {
		desc.KeySchema = append(desc.KeySchema, KeySchema{
			AttributeName: aws.ToString(ks.AttributeName),
			KeyType:       string(ks.KeyType),
		})
	}

	for _, ad := range t.AttributeDefinitions {
		desc.AttributeDefinitions = append(desc.AttributeDefinitions, AttributeDefinition{
			Name: aws.ToString(ad.AttributeName),
			Type: string(ad.AttributeType),
		})
	}

	for _, gsi := range t.GlobalSecondaryIndexes {
		idx := IndexDescription{
			Name:   aws.ToString(gsi.IndexName),
			Status: string(gsi.IndexStatus),
		}
		for _, ks := range gsi.KeySchema {
			idx.KeySchema = append(idx.KeySchema, KeySchema{
				AttributeName: aws.ToString(ks.AttributeName),
				KeyType:       string(ks.KeyType),
			})
		}
		desc.GlobalSecondaryIndexes = append(desc.GlobalSecondaryIndexes, idx)
	}

	return desc, nil
}

// Scan performs a table scan and returns items as simplified string maps.
func (c *Client) Scan(ctx context.Context, tableName string, exclusiveStartKey map[string]types.AttributeValue, limit int32) (*ScanResult, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(limit),
	}
	if exclusiveStartKey != nil {
		input.ExclusiveStartKey = exclusiveStartKey
	}

	out, err := c.api.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scan table: %w", err)
	}

	items := make([]Item, 0, len(out.Items))
	for _, rawItem := range out.Items {
		item := make(Item)
		for k, v := range rawItem {
			item[k] = formatAttributeValue(v)
		}
		items = append(items, item)
	}

	result := &ScanResult{
		Items:        items,
		ScannedCount: out.ScannedCount,
	}

	if out.LastEvaluatedKey != nil {
		result.LastEvaluatedKey = make(map[string]interface{})
		for k := range out.LastEvaluatedKey {
			result.LastEvaluatedKey[k] = out.LastEvaluatedKey[k]
		}
	}

	return result, nil
}

// RawLastEvaluatedKey converts the generic map back to DynamoDB attribute values.
// This is used for pagination continuations.
func RawLastEvaluatedKey(m map[string]interface{}) map[string]types.AttributeValue {
	if m == nil {
		return nil
	}
	result := make(map[string]types.AttributeValue, len(m))
	for k, v := range m {
		if av, ok := v.(types.AttributeValue); ok {
			result[k] = av
		}
	}
	return result
}

// ItemKeys returns the sorted keys from an item.
func ItemKeys(item Item) []string {
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatAttributeValue converts a DynamoDB attribute value to a display string.
func formatAttributeValue(av types.AttributeValue) string {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	case *types.AttributeValueMemberBOOL:
		return strconv.FormatBool(v.Value)
	case *types.AttributeValueMemberNULL:
		return "NULL"
	case *types.AttributeValueMemberB:
		return fmt.Sprintf("<binary %d bytes>", len(v.Value))
	case *types.AttributeValueMemberSS:
		return fmt.Sprintf("[%s]", joinStrings(v.Value))
	case *types.AttributeValueMemberNS:
		return fmt.Sprintf("[%s]", joinStrings(v.Value))
	case *types.AttributeValueMemberL:
		items := make([]string, 0, len(v.Value))
		for _, item := range v.Value {
			items = append(items, formatAttributeValue(item))
		}
		return fmt.Sprintf("[%s]", joinStrings(items))
	case *types.AttributeValueMemberM:
		parts := make([]string, 0, len(v.Value))
		keys := make([]string, 0, len(v.Value))
		for k := range v.Value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %s", k, formatAttributeValue(v.Value[k])))
		}
		return fmt.Sprintf("{%s}", joinStrings(parts))
	default:
		return "?"
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
