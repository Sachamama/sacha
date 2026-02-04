package s3

import "github.com/sachamama/sacha/internal/s3"

// bucketsLoadedMsg is sent when bucket listing completes.
type bucketsLoadedMsg struct {
	buckets []s3.Bucket
	err     error
}

// objectsLoadedMsg is sent when object listing completes.
type objectsLoadedMsg struct {
	objects   []s3.Object
	nextToken *string // for pagination
	err       error
}

// moreObjectsLoadedMsg is sent when additional objects are loaded (lazy loading).
type moreObjectsLoadedMsg struct {
	objects   []s3.Object
	nextToken *string
	err       error
}

// allObjectsLoadedMsg is sent when all remaining objects are loaded.
type allObjectsLoadedMsg struct {
	objects []s3.Object
	err     error
}

// detailsLoadedMsg is sent when object details are fetched.
type detailsLoadedMsg struct {
	details *s3.ObjectDetails
	key     string // key to match against current cursor
	err     error
}

// previewLoadedMsg is sent when object preview content is fetched.
type previewLoadedMsg struct {
	content     []byte
	contentType string
	key         string // key to match against current cursor
	err         error
}

// downloadCompleteMsg is sent when a download completes.
type downloadCompleteMsg struct {
	path string
	err  error
}

// bucketRegionMsg is sent when bucket region is fetched.
type bucketRegionMsg struct {
	bucket string
	region string
	err    error
}
