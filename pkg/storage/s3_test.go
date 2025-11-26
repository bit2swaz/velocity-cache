package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestS3IntegrationWithMinIO(t *testing.T) {
	endpoint := os.Getenv("LOCAL_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("LOCAL_S3_ENDPOINT not set; skipping MinIO integration test")
	}

	if os.Getenv("R2_ACCESS_KEY_ID") == "" || os.Getenv("R2_SECRET_ACCESS_KEY") == "" {
		t.Skip("R2_ACCESS_KEY_ID or R2_SECRET_ACCESS_KEY not set; skipping MinIO integration test")
	}

	bucket := os.Getenv("LOCAL_S3_BUCKET")
	if bucket == "" {
		bucket = "velocity-cache-integration"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewS3Client(ctx, bucket)
	if err != nil {
		t.Fatalf("new s3 client: %v", err)
	}

	if err := ensureBucket(ctx, client, bucket); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	payload := []byte("integration-payload-" + time.Now().Format(time.RFC3339Nano))
	cacheKey := fmt.Sprintf("integration/%d.zip", time.Now().UnixNano())

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "payload.zip")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	uploadCtx, uploadCancel := context.WithTimeout(ctx, 10*time.Second)
	defer uploadCancel()

	uploadResult := client.uploadRemote(uploadCtx, cacheKey, srcPath)
	if err := <-uploadResult; err != nil {
		t.Fatalf("upload remote: %v", err)
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()
	exists, err := client.checkRemote(checkCtx, cacheKey)
	if err != nil {
		t.Fatalf("check remote: %v", err)
	}
	if !exists {
		t.Fatalf("expected %s to exist in bucket", cacheKey)
	}

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.zip")

	downloadCtx, downloadCancel := context.WithTimeout(ctx, 10*time.Second)
	defer downloadCancel()
	if err := client.downloadRemote(downloadCtx, cacheKey, destPath); err != nil {
		t.Fatalf("download remote: %v", err)
	}

	downloaded, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}

	wantHash := sha256.Sum256(payload)
	gotHash := sha256.Sum256(downloaded)
	wantHex := hex.EncodeToString(wantHash[:])
	gotHex := hex.EncodeToString(gotHash[:])
	if wantHex != gotHex {
		t.Fatalf("hash mismatch: want %s got %s", wantHex, gotHex)
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()
	if err := deleteObject(cleanupCtx, client, cacheKey); err != nil {
		t.Logf("WARN: failed to delete integration object %s: %v", cacheKey, err)
	}
}

func ensureBucket(ctx context.Context, client *S3Client, bucket string) error {
	_, err := client.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}

	var exists *types.BucketAlreadyExists
	var owned *types.BucketAlreadyOwnedByYou
	if errors.As(err, &exists) || errors.As(err, &owned) {
		return nil
	}

	return fmt.Errorf("create bucket %s: %w", bucket, err)
}

func deleteObject(ctx context.Context, client *S3Client, key string) error {
	_, err := client.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(client.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("delete object %s: %w", key, err)
	}

	return nil
}
