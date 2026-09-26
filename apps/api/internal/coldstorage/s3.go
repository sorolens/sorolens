package coldstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config configures an S3-compatible bucket. Only Bucket is required; the
// remaining fields make MinIO, Backblaze B2, and other S3-compatible services
// work, since they need an explicit endpoint and path-style addressing.
type S3Config struct {
	// Bucket is the target bucket name. Required.
	Bucket string
	// Region is the signing region. Defaults to us-east-1, which is what most
	// S3-compatible services expect when they do not care.
	Region string
	// Endpoint overrides the AWS endpoint, e.g. http://localhost:9000 for
	// MinIO. Empty means the standard AWS endpoint for the region.
	Endpoint string
	// AccessKeyID and SecretAccessKey are optional explicit credentials. When
	// empty the default AWS credential chain is used. Supplying them keeps the
	// SDK from probing instance metadata, which is important in tests and
	// short-lived jobs.
	AccessKeyID     string
	SecretAccessKey string
}

// S3Store is an ObjectStore backed by an S3-compatible bucket.
type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store builds an S3Store from cfg.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("coldstorage: bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("coldstorage: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// Path-style addressing is what MinIO and most self-hosted
		// S3-compatible services require.
		o.UsePathStyle = true
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &S3Store{client: client, bucket: cfg.Bucket}, nil
}

// Put implements ObjectStore.
func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String("application/vnd.apache.parquet"),
	})
	if err != nil {
		return fmt.Errorf("coldstorage: put %s: %w", key, err)
	}
	return nil
}

// Get implements ObjectStore.
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrObjectNotFound, key)
		}
		return nil, fmt.Errorf("coldstorage: get %s: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("coldstorage: read %s: %w", key, err)
	}
	return data, nil
}

// List implements ObjectStore. It follows continuation tokens so buckets with
// more than 1000 archived keys are listed completely.
func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	var token *string
	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("coldstorage: list %s: %w", prefix, err)
		}
		for _, obj := range out.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}
	return keys, nil
}

// Delete implements ObjectStore. S3 itself treats deleting a missing key as a
// success, so no not-found handling is needed here.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("coldstorage: delete %s: %w", key, err)
	}
	return nil
}

// isNotFound reports whether an S3 error means "no such object". The SDK
// surfaces this as either a typed NoSuchKey/NotFound or a generic API error
// depending on which S3-compatible backend answered, so both shapes and a
// status-code fallback are checked.
func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "NoSuchKey") || strings.Contains(msg, "StatusCode: 404")
}
