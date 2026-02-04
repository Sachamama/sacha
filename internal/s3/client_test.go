package s3

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3API is a mock implementation of S3API for testing.
type mockS3API struct {
	listBucketsFn      func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	listObjectsV2Fn    func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	headObjectFn       func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	getObjectFn        func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	getBucketLocationFn func(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

func (m *mockS3API) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	if m.listBucketsFn != nil {
		return m.listBucketsFn(ctx, params, optFns...)
	}
	return &s3.ListBucketsOutput{}, nil
}

func (m *mockS3API) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsV2Fn != nil {
		return m.listObjectsV2Fn(ctx, params, optFns...)
	}
	return &s3.ListObjectsV2Output{}, nil
}

func (m *mockS3API) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObjectFn != nil {
		return m.headObjectFn(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

func (m *mockS3API) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn != nil {
		return m.getObjectFn(ctx, params, optFns...)
	}
	return &s3.GetObjectOutput{}, nil
}

func (m *mockS3API) GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if m.getBucketLocationFn != nil {
		return m.getBucketLocationFn(ctx, params, optFns...)
	}
	return &s3.GetBucketLocationOutput{}, nil
}

func TestListBuckets(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		mockFn      func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
		wantBuckets []Bucket
		wantErr     bool
	}{
		{
			name: "empty response",
			mockFn: func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
				return &s3.ListBucketsOutput{
					Buckets: []types.Bucket{},
				}, nil
			},
			wantBuckets: []Bucket{},
			wantErr:     false,
		},
		{
			name: "single bucket",
			mockFn: func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
				return &s3.ListBucketsOutput{
					Buckets: []types.Bucket{
						{
							Name:         aws.String("my-bucket"),
							CreationDate: aws.Time(now),
						},
					},
				}, nil
			},
			wantBuckets: []Bucket{
				{Name: "my-bucket", CreationDate: now},
			},
			wantErr: false,
		},
		{
			name: "multiple buckets",
			mockFn: func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
				return &s3.ListBucketsOutput{
					Buckets: []types.Bucket{
						{Name: aws.String("bucket-a"), CreationDate: aws.Time(now)},
						{Name: aws.String("bucket-b"), CreationDate: aws.Time(now.Add(-time.Hour))},
						{Name: aws.String("bucket-c"), CreationDate: aws.Time(now.Add(-2 * time.Hour))},
					},
				}, nil
			},
			wantBuckets: []Bucket{
				{Name: "bucket-a", CreationDate: now},
				{Name: "bucket-b", CreationDate: now.Add(-time.Hour)},
				{Name: "bucket-c", CreationDate: now.Add(-2 * time.Hour)},
			},
			wantErr: false,
		},
		{
			name: "API error",
			mockFn: func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				api:     &mockS3API{listBucketsFn: tt.mockFn},
				regions: make(map[string]string),
				clients: make(map[string]S3API),
			}

			buckets, err := client.ListBuckets(context.Background())

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListBuckets() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(buckets) != len(tt.wantBuckets) {
				t.Fatalf("got %d buckets, want %d", len(buckets), len(tt.wantBuckets))
			}

			for i, b := range buckets {
				if b.Name != tt.wantBuckets[i].Name {
					t.Errorf("bucket[%d].Name = %q, want %q", i, b.Name, tt.wantBuckets[i].Name)
				}
				if !b.CreationDate.Equal(tt.wantBuckets[i].CreationDate) {
					t.Errorf("bucket[%d].CreationDate = %v, want %v", i, b.CreationDate, tt.wantBuckets[i].CreationDate)
				}
			}
		})
	}
}

func TestListObjects(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		bucket      string
		prefix      string
		token       *string
		mockFn      func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
		wantObjects []Object
		wantToken   *string
		wantErr     bool
	}{
		{
			name:   "empty bucket",
			bucket: "my-bucket",
			prefix: "",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{
					Contents:       []types.Object{},
					CommonPrefixes: []types.CommonPrefix{},
				}, nil
			},
			wantObjects: []Object{},
			wantErr:     false,
		},
		{
			name:   "folders and files",
			bucket: "my-bucket",
			prefix: "",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				if *params.Delimiter != "/" {
					t.Errorf("expected delimiter '/', got %q", *params.Delimiter)
				}
				return &s3.ListObjectsV2Output{
					CommonPrefixes: []types.CommonPrefix{
						{Prefix: aws.String("folder1/")},
						{Prefix: aws.String("folder2/")},
					},
					Contents: []types.Object{
						{
							Key:          aws.String("file1.txt"),
							Size:         aws.Int64(1024),
							LastModified: aws.Time(now),
							StorageClass: types.ObjectStorageClassStandard,
						},
					},
				}, nil
			},
			wantObjects: []Object{
				{Key: "folder1/", Name: "folder1", IsPrefix: true},
				{Key: "folder2/", Name: "folder2", IsPrefix: true},
				{Key: "file1.txt", Name: "file1.txt", Size: 1024, LastModified: now, StorageClass: "STANDARD", IsPrefix: false},
			},
			wantErr: false,
		},
		{
			name:   "nested prefix",
			bucket: "my-bucket",
			prefix: "folder1/",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				if *params.Prefix != "folder1/" {
					t.Errorf("expected prefix 'folder1/', got %q", *params.Prefix)
				}
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:          aws.String("folder1/nested.txt"),
							Size:         aws.Int64(512),
							LastModified: aws.Time(now),
						},
					},
				}, nil
			},
			wantObjects: []Object{
				{Key: "folder1/nested.txt", Name: "nested.txt", Size: 512, LastModified: now, IsPrefix: false},
			},
			wantErr: false,
		},
		{
			name:   "skips prefix itself as object",
			bucket: "my-bucket",
			prefix: "folder1/",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("folder1/"), Size: aws.Int64(0)}, // prefix marker
						{Key: aws.String("folder1/file.txt"), Size: aws.Int64(100)},
					},
				}, nil
			},
			wantObjects: []Object{
				{Key: "folder1/file.txt", Name: "file.txt", Size: 100, IsPrefix: false},
			},
			wantErr: false,
		},
		{
			name:   "pagination",
			bucket: "my-bucket",
			prefix: "",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("file1.txt"), Size: aws.Int64(100)},
					},
					IsTruncated:           aws.Bool(true),
					NextContinuationToken: aws.String("token123"),
				}, nil
			},
			wantObjects: []Object{
				{Key: "file1.txt", Name: "file1.txt", Size: 100, IsPrefix: false},
			},
			wantToken: aws.String("token123"),
			wantErr:   false,
		},
		{
			name:   "uses continuation token",
			bucket: "my-bucket",
			prefix: "",
			token:  aws.String("continue-from-here"),
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				if params.ContinuationToken == nil || *params.ContinuationToken != "continue-from-here" {
					t.Errorf("expected continuation token 'continue-from-here', got %v", params.ContinuationToken)
				}
				return &s3.ListObjectsV2Output{}, nil
			},
			wantObjects: []Object{},
			wantErr:     false,
		},
		{
			name:   "API error",
			bucket: "my-bucket",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return nil, errors.New("bucket not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{listObjectsV2Fn: tt.mockFn}
			client := &Client{
				api:     mockAPI,
				regions: map[string]string{tt.bucket: "us-east-1"},
				clients: map[string]S3API{"us-east-1": mockAPI},
			}

			objects, token, err := client.ListObjects(context.Background(), tt.bucket, tt.prefix, tt.token)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListObjects() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(objects) != len(tt.wantObjects) {
				t.Fatalf("got %d objects, want %d", len(objects), len(tt.wantObjects))
			}

			for i, o := range objects {
				want := tt.wantObjects[i]
				if o.Key != want.Key {
					t.Errorf("object[%d].Key = %q, want %q", i, o.Key, want.Key)
				}
				if o.Name != want.Name {
					t.Errorf("object[%d].Name = %q, want %q", i, o.Name, want.Name)
				}
				if o.IsPrefix != want.IsPrefix {
					t.Errorf("object[%d].IsPrefix = %v, want %v", i, o.IsPrefix, want.IsPrefix)
				}
				if o.Size != want.Size {
					t.Errorf("object[%d].Size = %d, want %d", i, o.Size, want.Size)
				}
			}

			if (token == nil) != (tt.wantToken == nil) {
				t.Errorf("token = %v, wantToken = %v", token, tt.wantToken)
			}
		})
	}
}

func TestGetObjectDetails(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		bucket  string
		key     string
		mockFn  func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
		want    *ObjectDetails
		wantErr bool
	}{
		{
			name:   "successful retrieval",
			bucket: "my-bucket",
			key:    "path/to/file.json",
			mockFn: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
				if *params.Bucket != "my-bucket" {
					t.Errorf("unexpected bucket: %s", *params.Bucket)
				}
				if *params.Key != "path/to/file.json" {
					t.Errorf("unexpected key: %s", *params.Key)
				}
				return &s3.HeadObjectOutput{
					ContentLength: aws.Int64(2048),
					ContentType:   aws.String("application/json"),
					LastModified:  aws.Time(now),
					StorageClass:  types.StorageClassStandard,
					ETag:          aws.String("\"abc123\""),
				}, nil
			},
			want: &ObjectDetails{
				Object: Object{
					Key:          "path/to/file.json",
					Name:         "path/to/file.json", // extractName with empty prefix returns full key
					Size:         2048,
					LastModified: now,
					StorageClass: "STANDARD",
					IsPrefix:     false,
				},
				ContentType: "application/json",
				ETag:        "\"abc123\"",
			},
			wantErr: false,
		},
		{
			name:   "API error",
			bucket: "my-bucket",
			key:    "nonexistent.txt",
			mockFn: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
				return nil, errors.New("not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{headObjectFn: tt.mockFn}
			client := &Client{
				api:     mockAPI,
				regions: map[string]string{tt.bucket: "us-east-1"},
				clients: map[string]S3API{"us-east-1": mockAPI},
			}

			details, err := client.GetObjectDetails(context.Background(), tt.bucket, tt.key)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetObjectDetails() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if details.Key != tt.want.Key {
				t.Errorf("Key = %q, want %q", details.Key, tt.want.Key)
			}
			if details.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", details.Name, tt.want.Name)
			}
			if details.Size != tt.want.Size {
				t.Errorf("Size = %d, want %d", details.Size, tt.want.Size)
			}
			if details.ContentType != tt.want.ContentType {
				t.Errorf("ContentType = %q, want %q", details.ContentType, tt.want.ContentType)
			}
			if details.ETag != tt.want.ETag {
				t.Errorf("ETag = %q, want %q", details.ETag, tt.want.ETag)
			}
		})
	}
}

func TestGetObjectPreview(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		maxBytes    int64
		mockFn      func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		wantContent string
		wantType    string
		wantErr     bool
	}{
		{
			name:     "successful preview",
			bucket:   "my-bucket",
			key:      "file.txt",
			maxBytes: 1024,
			mockFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				if params.Range == nil || *params.Range != "bytes=0-1023" {
					t.Errorf("expected range 'bytes=0-1023', got %v", params.Range)
				}
				return &s3.GetObjectOutput{
					Body:        io.NopCloser(strings.NewReader("Hello, World!")),
					ContentType: aws.String("text/plain"),
				}, nil
			},
			wantContent: "Hello, World!",
			wantType:    "text/plain",
			wantErr:     false,
		},
		{
			name:     "API error",
			bucket:   "my-bucket",
			key:      "file.txt",
			maxBytes: 1024,
			mockFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{getObjectFn: tt.mockFn}
			client := &Client{
				api:     mockAPI,
				regions: map[string]string{tt.bucket: "us-east-1"},
				clients: map[string]S3API{"us-east-1": mockAPI},
			}

			content, contentType, err := client.GetObjectPreview(context.Background(), tt.bucket, tt.key, tt.maxBytes)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetObjectPreview() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if string(content) != tt.wantContent {
				t.Errorf("content = %q, want %q", string(content), tt.wantContent)
			}
			if contentType != tt.wantType {
				t.Errorf("contentType = %q, want %q", contentType, tt.wantType)
			}
		})
	}
}

func TestDownloadObject(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		key      string
		content  string
		mockFn   func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		wantErr  bool
	}{
		{
			name:    "successful download",
			bucket:  "my-bucket",
			key:     "file.txt",
			content: "file content here",
			mockFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body:          io.NopCloser(strings.NewReader("file content here")),
					ContentLength: aws.Int64(17),
				}, nil
			},
			wantErr: false,
		},
		{
			name:   "API error",
			bucket: "my-bucket",
			key:    "file.txt",
			mockFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, errors.New("not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{getObjectFn: tt.mockFn}
			client := &Client{
				api:     mockAPI,
				regions: map[string]string{tt.bucket: "us-east-1"},
				clients: map[string]S3API{"us-east-1": mockAPI},
			}

			tmpDir := t.TempDir()
			destPath := filepath.Join(tmpDir, "downloaded.txt")

			err := client.DownloadObject(context.Background(), tt.bucket, tt.key, destPath)

			if (err != nil) != tt.wantErr {
				t.Fatalf("DownloadObject() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			// Verify file was written correctly
			data, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("failed to read downloaded file: %v", err)
			}
			if string(data) != tt.content {
				t.Errorf("file content = %q, want %q", string(data), tt.content)
			}
		})
	}
}

func TestDownloadObjectWithProgress(t *testing.T) {
	mockAPI := &mockS3API{
		getObjectFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			content := "0123456789" // 10 bytes
			return &s3.GetObjectOutput{
				Body:          io.NopCloser(strings.NewReader(content)),
				ContentLength: aws.Int64(int64(len(content))),
			}, nil
		},
	}
	client := &Client{
		api:     mockAPI,
		regions: map[string]string{"my-bucket": "us-east-1"},
		clients: map[string]S3API{"us-east-1": mockAPI},
	}

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.txt")

	var progressCalls []int64
	progressFn := func(written, total int64) {
		progressCalls = append(progressCalls, written)
	}

	err := client.DownloadObjectWithProgress(context.Background(), "my-bucket", "file.txt", destPath, progressFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(progressCalls) == 0 {
		t.Error("progress function was never called")
	}

	// Last progress call should equal total size
	if len(progressCalls) > 0 && progressCalls[len(progressCalls)-1] != 10 {
		t.Errorf("final progress = %d, want 10", progressCalls[len(progressCalls)-1])
	}
}

func TestDownloadObjectCreatesDirectory(t *testing.T) {
	mockAPI := &mockS3API{
		getObjectFn: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body:          io.NopCloser(strings.NewReader("content")),
				ContentLength: aws.Int64(7),
			}, nil
		},
	}
	client := &Client{
		api:     mockAPI,
		regions: map[string]string{"my-bucket": "us-east-1"},
		clients: map[string]S3API{"us-east-1": mockAPI},
	}

	tmpDir := t.TempDir()
	// Use nested path that doesn't exist
	destPath := filepath.Join(tmpDir, "nested", "dir", "file.txt")

	err := client.DownloadObject(context.Background(), "my-bucket", "key", destPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func TestListAllObjects(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		bucket      string
		prefix      string
		mockFn      func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
		wantObjects int
		wantErr     bool
	}{
		{
			name:   "single page",
			bucket: "my-bucket",
			prefix: "prefix/",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				// Verify no delimiter (recursive listing)
				if params.Delimiter != nil {
					t.Errorf("expected no delimiter for recursive listing, got %v", params.Delimiter)
				}
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("prefix/file1.txt"), Size: aws.Int64(100), LastModified: aws.Time(now)},
						{Key: aws.String("prefix/file2.txt"), Size: aws.Int64(200), LastModified: aws.Time(now)},
					},
					IsTruncated: aws.Bool(false),
				}, nil
			},
			wantObjects: 2,
			wantErr:     false,
		},
		{
			name:   "multiple pages",
			bucket: "my-bucket",
			prefix: "prefix/",
			mockFn: func() func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				callCount := 0
				return func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					callCount++
					if callCount == 1 {
						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{Key: aws.String("prefix/file1.txt"), Size: aws.Int64(100)},
							},
							IsTruncated:           aws.Bool(true),
							NextContinuationToken: aws.String("page2"),
						}, nil
					}
					return &s3.ListObjectsV2Output{
						Contents: []types.Object{
							{Key: aws.String("prefix/file2.txt"), Size: aws.Int64(200)},
						},
						IsTruncated: aws.Bool(false),
					}, nil
				}
			}(),
			wantObjects: 2,
			wantErr:     false,
		},
		{
			name:   "skips folder markers",
			bucket: "my-bucket",
			prefix: "prefix/",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("prefix/"), Size: aws.Int64(0)},         // folder marker
						{Key: aws.String("prefix/subfolder/"), Size: aws.Int64(0)}, // folder marker
						{Key: aws.String("prefix/file.txt"), Size: aws.Int64(100)},
					},
					IsTruncated: aws.Bool(false),
				}, nil
			},
			wantObjects: 1, // only the actual file
			wantErr:     false,
		},
		{
			name:   "API error",
			bucket: "my-bucket",
			prefix: "",
			mockFn: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{listObjectsV2Fn: tt.mockFn}
			client := &Client{
				api:     mockAPI,
				regions: map[string]string{tt.bucket: "us-east-1"},
				clients: map[string]S3API{"us-east-1": mockAPI},
			}

			objects, err := client.ListAllObjects(context.Background(), tt.bucket, tt.prefix)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListAllObjects() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(objects) != tt.wantObjects {
				t.Errorf("got %d objects, want %d", len(objects), tt.wantObjects)
			}
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		key    string
		prefix string
		want   string
	}{
		{"file.txt", "", "file.txt"},
		{"folder/file.txt", "folder/", "file.txt"},
		{"a/b/c/file.txt", "a/b/", "c/file.txt"},
		{"folder/", "", "folder"},
		{"a/b/c/", "a/b/", "c"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := extractName(tt.key, tt.prefix)
			if got != tt.want {
				t.Errorf("extractName(%q, %q) = %q, want %q", tt.key, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestBucketRegionCaching(t *testing.T) {
	client := &Client{
		regions: make(map[string]string),
		clients: make(map[string]S3API),
	}

	// Pre-populate cache
	client.regions["cached-bucket"] = "eu-west-1"

	// Verify cache hit
	region, ok := client.regions["cached-bucket"]
	if !ok || region != "eu-west-1" {
		t.Errorf("expected cached region 'eu-west-1', got %q", region)
	}

	// Verify cache miss
	_, ok = client.regions["uncached-bucket"]
	if ok {
		t.Error("expected cache miss for uncached-bucket")
	}
}
