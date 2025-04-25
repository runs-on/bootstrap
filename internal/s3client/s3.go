package s3client

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GetObjectAPI defines the interface for the GetObject function.
type GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Download downloads an object from S3 using the provided client
func Download(ctx context.Context, api GetObjectAPI, bucket, key string) (io.ReadCloser, error) {
	result, err := api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting object from S3: %w", err)
	}
	return result.Body, nil
}
