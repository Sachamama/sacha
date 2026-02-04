package s3

import "time"

// Bucket represents an S3 bucket.
type Bucket struct {
	Name         string
	CreationDate time.Time
}

// Object represents an S3 object or prefix (folder).
type Object struct {
	Key          string
	Name         string // display name (last path segment)
	Size         int64
	LastModified time.Time
	StorageClass string
	IsPrefix     bool // true for "folders"
}

// ObjectDetails contains extended metadata for an S3 object.
type ObjectDetails struct {
	Object
	ContentType string
	ETag        string
}
