package dynamodb

import "github.com/sachamama/sacha/internal/dynamodb"

// tablesLoadedMsg is sent when table listing completes.
type tablesLoadedMsg struct {
	tables    []dynamodb.Table
	nextToken *string
	err       error
}

// moreTablesLoadedMsg is sent when additional tables are loaded.
type moreTablesLoadedMsg struct {
	tables    []dynamodb.Table
	nextToken *string
	err       error
}

// tableDescriptionMsg is sent when table description is fetched.
type tableDescriptionMsg struct {
	desc      *dynamodb.TableDescription
	tableName string
	err       error
}

// itemsLoadedMsg is sent when a scan completes.
type itemsLoadedMsg struct {
	result    *dynamodb.ScanResult
	tableName string
	err       error
}

// moreItemsLoadedMsg is sent when additional items are loaded.
type moreItemsLoadedMsg struct {
	result *dynamodb.ScanResult
	err    error
}

// allTablesLoadedMsg is sent when all remaining tables are loaded.
type allTablesLoadedMsg struct {
	tables []dynamodb.Table
	err    error
}

// allItemsLoadedMsg is sent when all remaining items are loaded.
type allItemsLoadedMsg struct {
	result *dynamodb.ScanResult
	err    error
}
