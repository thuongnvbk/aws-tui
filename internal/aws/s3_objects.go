package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Object represents an object or prefix in an S3 bucket
type S3Object struct {
	Key          string
	Size         int64
	LastModified string
	StorageClass string
	ETag         string
	IsPrefix     bool // true for folders/prefixes
	ContentType  string
}

// S3ObjectDetail contains detailed information about an S3 object
type S3ObjectDetail struct {
	S3Object
	VersionId            string
	Metadata             map[string]string
	Tags                 map[string]string
	ServerSideEncryption string
	ContentPreview       string // first N bytes as string
	IsPreviewable        bool
}

// ListObjects lists objects in an S3 bucket with optional prefix and delimiter
func (c *Client) ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxKeys int32) ([]S3Object, error) {
	if maxKeys == 0 {
		maxKeys = 1000
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		MaxKeys:   aws.Int32(maxKeys),
		Delimiter: aws.String(delimiter),
	}

	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	var objects []S3Object
	paginator := s3.NewListObjectsV2Paginator(c.S3, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
		}

		// Add prefixes (folders)
		for _, prefix := range output.CommonPrefixes {
			objects = append(objects, S3Object{
				Key:      aws.ToString(prefix.Prefix),
				IsPrefix: true,
			})
		}

		// Add objects (files)
		for _, obj := range output.Contents {
			key := aws.ToString(obj.Key)
			// Skip if it's the prefix itself (empty folder marker)
			if key == prefix {
				continue
			}

			object := S3Object{
				Key:          key,
				Size:         aws.ToInt64(obj.Size),
				StorageClass: string(obj.StorageClass),
				ETag:         aws.ToString(obj.ETag),
				IsPrefix:     false,
			}

			if obj.LastModified != nil {
				object.LastModified = obj.LastModified.Format(c.dateFmt)
			}

			objects = append(objects, object)
		}
	}

	return objects, nil
}

// GetObjectMetadata retrieves detailed metadata for an S3 object
func (c *Client) GetObjectMetadata(ctx context.Context, bucket, key string) (*S3ObjectDetail, error) {
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	headOutput, err := c.S3.HeadObject(ctx, headInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	detail := &S3ObjectDetail{
		S3Object: S3Object{
			Key:          key,
			Size:         aws.ToInt64(headOutput.ContentLength),
			StorageClass: string(headOutput.StorageClass),
			ETag:         aws.ToString(headOutput.ETag),
			ContentType:  aws.ToString(headOutput.ContentType),
		},
		VersionId:            aws.ToString(headOutput.VersionId),
		Metadata:             headOutput.Metadata,
		ServerSideEncryption: string(headOutput.ServerSideEncryption),
	}

	if headOutput.LastModified != nil {
		detail.LastModified = headOutput.LastModified.Format(c.dateFmt)
	}

	// Get tags
	tags, err := c.getObjectTags(ctx, bucket, key)
	if err == nil {
		detail.Tags = tags
	}

	return detail, nil
}

// getObjectTags retrieves tags for an S3 object
func (c *Client) getObjectTags(ctx context.Context, bucket, key string) (map[string]string, error) {
	input := &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	output, err := c.S3.GetObjectTagging(ctx, input)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	for _, tag := range output.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags, nil
}

// GetObjectContent retrieves the content of an S3 object up to maxSize bytes
func (c *Client) GetObjectContent(ctx context.Context, bucket, key string, maxSize int64) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// If maxSize is specified, use Range header
	if maxSize > 0 {
		input.Range = aws.String(fmt.Sprintf("bytes=0-%d", maxSize-1))
	}

	output, err := c.S3.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object content: %w", err)
	}
	defer output.Body.Close()

	content, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return content, nil
}

// DownloadObject downloads an S3 object to a local file
func (c *Client) DownloadObject(ctx context.Context, bucket, key, localPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get object
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	output, err := c.S3.GetObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer output.Body.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Copy content
	_, err = io.Copy(file, output.Body)
	if err != nil {
		return fmt.Errorf("failed to write to local file: %w", err)
	}

	return nil
}

// GeneratePresignedURL generates a presigned URL for an S3 object
func (c *Client) GeneratePresignedURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.S3)

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	presignedURL, err := presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.URL, nil
}

// GetBucketLocation retrieves the region of an S3 bucket
func (c *Client) GetBucketLocation(ctx context.Context, bucket string) (string, error) {
	input := &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	}

	output, err := c.S3.GetBucketLocation(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get bucket location: %w", err)
	}

	location := string(output.LocationConstraint)
	if location == "" {
		location = "us-east-1" // Default region
	}

	return location, nil
}

// IsPreviewableContent checks if content type is previewable
func IsPreviewableContent(contentType string, supportedTypes []string) bool {
	if len(supportedTypes) == 0 {
		// Default previewable types
		supportedTypes = []string{
			"text/plain",
			"application/json",
			"application/yaml",
			"application/x-yaml",
			"text/yaml",
			"text/csv",
			"text/html",
			"text/xml",
			"application/xml",
			"text/markdown",
		}
	}

	contentType = strings.ToLower(strings.TrimSpace(contentType))
	for _, supported := range supportedTypes {
		if strings.HasPrefix(contentType, strings.ToLower(supported)) {
			return true
		}
	}

	return false
}

// FormatSize formats bytes into human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetObjectName extracts the object name from a full key path
func GetObjectName(key string) string {
	parts := strings.Split(strings.TrimSuffix(key, "/"), "/")
	if len(parts) == 0 {
		return key
	}
	return parts[len(parts)-1]
}

// GetParentPrefix returns the parent prefix of a given prefix
func GetParentPrefix(prefix string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	parts := strings.Split(prefix, "/")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/") + "/"
}

// BuildBreadcrumb builds a breadcrumb path from a prefix
func BuildBreadcrumb(bucket, prefix string) []string {
	breadcrumb := []string{bucket}
	if prefix == "" {
		return breadcrumb
	}

	parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	for _, part := range parts {
		if part != "" {
			breadcrumb = append(breadcrumb, part)
		}
	}

	return breadcrumb
}

// DetailLines returns formatted detail lines for an S3 object
func (obj *S3ObjectDetail) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Object Key:      %s", obj.Key),
		fmt.Sprintf("Size:            %s (%d bytes)", FormatSize(obj.Size), obj.Size),
		fmt.Sprintf("Last Modified:   %s", obj.LastModified),
		fmt.Sprintf("Storage Class:   %s", obj.StorageClass),
		fmt.Sprintf("Content Type:    %s", obj.ContentType),
		fmt.Sprintf("ETag:            %s", obj.ETag),
	}

	if obj.ServerSideEncryption != "" {
		lines = append(lines, fmt.Sprintf("Encryption:      %s", obj.ServerSideEncryption))
	}

	if obj.VersionId != "" {
		lines = append(lines, fmt.Sprintf("Version ID:      %s", obj.VersionId))
	}

	if len(obj.Metadata) > 0 {
		lines = append(lines, "", "Metadata:")
		for k, v := range obj.Metadata {
			lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
		}
	}

	if len(obj.Tags) > 0 {
		lines = append(lines, "", "Tags:")
		for k, v := range obj.Tags {
			lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
		}
	}

	return lines
}

// Made with Bob
