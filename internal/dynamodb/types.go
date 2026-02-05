package dynamodb

import "time"

// Table represents a DynamoDB table summary.
type Table struct {
	Name string
}

// KeySchema describes a key attribute in a DynamoDB table.
type KeySchema struct {
	AttributeName string
	KeyType       string // HASH or RANGE
}

// AttributeDefinition describes an attribute's name and type.
type AttributeDefinition struct {
	Name string
	Type string // S, N, B
}

// TableDescription contains detailed metadata about a DynamoDB table.
type TableDescription struct {
	Name                   string
	Status                 string
	ItemCount              int64
	TableSizeBytes         int64
	CreationDateTime       time.Time
	KeySchema              []KeySchema
	AttributeDefinitions   []AttributeDefinition
	BillingMode            string
	ReadCapacity           int64
	WriteCapacity          int64
	GlobalSecondaryIndexes []IndexDescription
}

// IndexDescription describes a Global Secondary Index.
type IndexDescription struct {
	Name      string
	Status    string
	KeySchema []KeySchema
}

// Item represents a DynamoDB item as a map of attribute name to display value.
type Item map[string]string

// ScanResult contains the result of a Scan operation.
type ScanResult struct {
	Items            []Item
	LastEvaluatedKey map[string]interface{}
	ScannedCount     int32
}
