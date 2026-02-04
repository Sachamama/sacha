package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3API captures the AWS SDK methods we use.
type S3API interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// Client wraps the S3 API for bucket and object operations.
type Client struct {
	cfg     aws.Config
	api     S3API
	mu      sync.RWMutex
	regions map[string]string // bucket -> region cache
	clients map[string]S3API  // region -> client cache
}

// NewClient creates a new S3 client from the provided AWS config.
func NewClient(cfg aws.Config) *Client {
	return &Client{
		cfg:     cfg,
		api:     s3.NewFromConfig(cfg),
		regions: make(map[string]string),
		clients: make(map[string]S3API),
	}
}

// ListBuckets returns all S3 buckets.
func (c *Client) ListBuckets(ctx context.Context) ([]Bucket, error) {
	out, err := c.api.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}

	buckets := make([]Bucket, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		buckets = append(buckets, Bucket{
			Name:         aws.ToString(b.Name),
			CreationDate: aws.ToTime(b.CreationDate),
		})
	}
	return buckets, nil
}

// getBucketRegion returns the region for a bucket, using cache when available.
func (c *Client) getBucketRegion(ctx context.Context, bucket string) (string, error) {
	c.mu.RLock()
	region, ok := c.regions[bucket]
	c.mu.RUnlock()
	if ok {
		return region, nil
	}

	// GetBucketLocation must be called from us-east-1 for reliable results
	usEast1Client := s3.NewFromConfig(c.cfg, func(o *s3.Options) {
		o.Region = "us-east-1"
	})

	out, err := usEast1Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return "", fmt.Errorf("get bucket location: %w", err)
	}

	// Empty location means us-east-1
	region = string(out.LocationConstraint)
	if region == "" {
		region = "us-east-1"
	}

	c.mu.Lock()
	c.regions[bucket] = region
	c.mu.Unlock()

	return region, nil
}

// getClientForBucket returns an S3 client configured for the bucket's region.
func (c *Client) getClientForBucket(ctx context.Context, bucket string) (S3API, error) {
	region, err := c.getBucketRegion(ctx, bucket)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	client, ok := c.clients[region]
	c.mu.RUnlock()
	if ok {
		return client, nil
	}

	client = s3.NewFromConfig(c.cfg, func(o *s3.Options) {
		o.Region = region
	})

	c.mu.Lock()
	c.clients[region] = client
	c.mu.Unlock()

	return client, nil
}

// ListObjects returns objects and common prefixes in a bucket with the given prefix.
// Uses "/" as delimiter for folder-like navigation.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string, token *string) ([]Object, *string, error) {
	api, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return nil, nil, err
	}

	out, err := api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucket),
		Prefix:            aws.String(prefix),
		Delimiter:         aws.String("/"),
		ContinuationToken: token,
		MaxKeys:           aws.Int32(1000),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list objects: %w", err)
	}

	objects := make([]Object, 0, len(out.CommonPrefixes)+len(out.Contents))

	// Add prefixes (folders) first
	for _, p := range out.CommonPrefixes {
		prefixStr := aws.ToString(p.Prefix)
		name := extractName(prefixStr, prefix)
		objects = append(objects, Object{
			Key:      prefixStr,
			Name:     name,
			IsPrefix: true,
		})
	}

	// Add objects (files)
	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		// Skip the prefix itself if it appears as an object
		if key == prefix {
			continue
		}
		name := extractName(key, prefix)
		objects = append(objects, Object{
			Key:          key,
			Name:         name,
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			StorageClass: string(obj.StorageClass),
			IsPrefix:     false,
		})
	}

	var nextToken *string
	if out.IsTruncated != nil && *out.IsTruncated {
		nextToken = out.NextContinuationToken
	}

	return objects, nextToken, nil
}

// GetObjectDetails retrieves extended metadata for an object.
func (c *Client) GetObjectDetails(ctx context.Context, bucket, key string) (*ObjectDetails, error) {
	api, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}

	out, err := api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head object: %w", err)
	}

	return &ObjectDetails{
		Object: Object{
			Key:          key,
			Name:         extractName(key, ""),
			Size:         aws.ToInt64(out.ContentLength),
			LastModified: aws.ToTime(out.LastModified),
			StorageClass: string(out.StorageClass),
			IsPrefix:     false,
		},
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}, nil
}

// GetObjectPreview downloads up to maxBytes of an object and returns the content with its content type.
func (c *Client) GetObjectPreview(ctx context.Context, bucket, key string, maxBytes int64) ([]byte, string, error) {
	api, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return nil, "", err
	}

	rangeHeader := fmt.Sprintf("bytes=0-%d", maxBytes-1)
	out, err := api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get object preview: %w", err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read object body: %w", err)
	}

	return data, aws.ToString(out.ContentType), nil
}

// DownloadObject downloads an object to the specified destination path.
func (c *Client) DownloadObject(ctx context.Context, bucket, key, destPath string) error {
	api, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	out, err := api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer func() { _ = out.Body.Close() }()

	// Ensure the destination directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, out.Body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write file: %w", err)
	}

	return f.Close()
}

// DeleteObjects deletes multiple objects from a bucket.
func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	api, err := c.getClientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	objectIDs := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objectIDs[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}

	_, err = api.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objectIDs,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("delete objects: %w", err)
	}

	return nil
}

// GetBucketRegion returns the region for a bucket.
func (c *Client) GetBucketRegion(ctx context.Context, bucket string) (string, error) {
	return c.getBucketRegion(ctx, bucket)
}

// extractName extracts the display name from a key given the current prefix.
func extractName(key, prefix string) string {
	name := strings.TrimPrefix(key, prefix)
	name = strings.TrimSuffix(name, "/")
	return name
}
