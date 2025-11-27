package s3

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Driver struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

func New(ctx context.Context) (*S3Driver, error) {
	bucket := os.Getenv("VC_S3_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("VC_S3_BUCKET is not set")
	}
	region := os.Getenv("VC_S3_REGION")
	if region == "" {
		return nil, fmt.Errorf("VC_S3_REGION is not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	endpoint := os.Getenv("VC_S3_ENDPOINT")

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	presignClient := s3.NewPresignClient(client)

	return &S3Driver{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
	}, nil
}

func (d *S3Driver) GetUploadURL(ctx context.Context, key string) (string, error) {
	req, err := d.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("failed to presign put object: %w", err)
	}
	return req.URL, nil
}

func (d *S3Driver) GetDownloadURL(ctx context.Context, key string) (string, error) {
	req, err := d.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("failed to presign get object: %w", err)
	}
	return req.URL, nil
}

func (d *S3Driver) Exists(ctx context.Context, key string) (bool, error) {
	_, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	})
	if err != nil {

		return false, nil
	}
	return true, nil
}
