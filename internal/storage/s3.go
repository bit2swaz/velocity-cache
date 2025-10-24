package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const defaultPublicBucket = "velocity-cache-mvp-public-1"

// S3Client wraps the S3 service client and managers.
type S3Client struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	bucketName string
}

// NewS3Client creates and configures a new S3 client.
// It will connect to a local MinIO instance if LOCAL_S3_ENDPOINT is set.
// Otherwise, it will connect to the production Cloudflare R2.
func NewS3Client(ctx context.Context, bucketName string) (*S3Client, error) {
	var cfg aws.Config
	var err error

	if strings.TrimSpace(bucketName) == "" {
		bucketName = defaultPublicBucket
	}

	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	localEndpoint := os.Getenv("LOCAL_S3_ENDPOINT")

	var forcePathStyle bool

	if localEndpoint != "" {
		log.Println("INFO: Using local MinIO endpoint:", localEndpoint)

		if accessKeyID == "" || secretKey == "" {
			return nil, errors.New("R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set for MinIO")
		}

		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               localEndpoint,
				HostnameImmutable: true,
				SigningRegion:     "us-east-1",
			}, nil
		})

		creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithEndpointResolverWithOptions(customResolver),
			config.WithCredentialsProvider(creds),
			config.WithRegion("us-east-1"),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load MinIO config: %w", err)
		}
		forcePathStyle = true
	} else {
		log.Println("INFO: Using production Cloudflare R2 endpoint")

		accountID := os.Getenv("R2_ACCOUNT_ID")
		if accountID == "" || accessKeyID == "" || secretKey == "" {
			return nil, errors.New("R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, and R2_SECRET_ACCESS_KEY must be set for R2")
		}

		r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               r2Endpoint,
				HostnameImmutable: true,
				SigningRegion:     "auto",
			}, nil
		})

		creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithEndpointResolverWithOptions(customResolver),
			config.WithCredentialsProvider(creds),
			config.WithRegion("auto"),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load R2 config: %w", err)
		}
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if forcePathStyle {
			o.UsePathStyle = true
		}
	})
	uploader := manager.NewUploader(client)
	downloader := manager.NewDownloader(client)

	if err := ensureBucketExists(ctx, client, bucketName); err != nil {
		return nil, fmt.Errorf("ensure bucket %s: %w", bucketName, err)
	}

	return &S3Client{
		client:     client,
		uploader:   uploader,
		downloader: downloader,
		bucketName: bucketName,
	}, nil
}

func ensureBucketExists(ctx context.Context, client *s3.Client, bucket string) error {
	if strings.TrimSpace(bucket) == "" {
		return errors.New("bucket name is empty")
	}

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}

	var exists *types.BucketAlreadyExists
	var owned *types.BucketAlreadyOwnedByYou
	if errors.As(err, &exists) || errors.As(err, &owned) {
		return nil
	}

	return err
}

func (c *S3Client) checkRemote(ctx context.Context, cacheKey string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(cacheKey),
	})

	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *S3Client) downloadRemote(ctx context.Context, cacheKey, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("download remote ensure dir %s: %w", filepath.Dir(localPath), err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer file.Close()

	_, err = c.downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(cacheKey),
	})
	if err != nil {
		return fmt.Errorf("download remote object %s: %w", cacheKey, err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("download remote sync %s: %w", localPath, err)
	}
	return nil
}

func (c *S3Client) uploadRemote(ctx context.Context, cacheKey, localPath string) <-chan error {
	result := make(chan error, 1)

	go func() {
		defer close(result)

		file, err := os.Open(localPath)
		if err != nil {
			result <- fmt.Errorf("open upload source %s: %w", localPath, err)
			return
		}
		defer file.Close()

		_, err = c.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(c.bucketName),
			Key:    aws.String(cacheKey),
			Body:   file,
		})
		if err != nil {
			result <- fmt.Errorf("upload remote object %s: %w", cacheKey, err)
			return
		}

		result <- nil
	}()

	return result
}

func (c *S3Client) CheckRemote(ctx context.Context, cacheKey string) (bool, error) {
	return c.checkRemote(ctx, cacheKey)
}

func (c *S3Client) DownloadRemote(ctx context.Context, cacheKey, localPath string) error {
	return c.downloadRemote(ctx, cacheKey, localPath)
}

func (c *S3Client) UploadRemote(ctx context.Context, cacheKey, localPath string) <-chan error {
	return c.uploadRemote(ctx, cacheKey, localPath)
}
