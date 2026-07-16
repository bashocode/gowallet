package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStorage interface {
	UploadStream(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error)
}

type minioStorage struct {
	client   *minio.Client
	endpoint string
	useSSL   bool
}

func NewMinioStorage(endpoint, accessKey, secretKey string, useSSL bool) (ObjectStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &minioStorage{
		client:   client,
		endpoint: endpoint,
		useSSL:   useSSL,
	}, nil
}

func (s *minioStorage) UploadStream(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	exists, err := s.client.BucketExists(ctx, bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create bucket: %w", err)
		}

		policy := fmt.Sprintf(`{
			"Version":"2012-10-17",
			"Statement":[{
				"Effect":"Allow",
				"Principal":{"AWS":["*"]},
				"Action":["s3:GetObject"],
				"Resource":["arn:aws:s3:::%s/*"]
			}]
		}`, bucketName)

		if err := s.client.SetBucketPolicy(ctx, bucketName, policy); err != nil {
			return "", fmt.Errorf("failed to set bucket policy: %w", err)
		}
	}

	_, err = s.client.PutObject(ctx, bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object: %w", err)
	}

	schema := "http"
	if s.useSSL {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", schema, s.endpoint, bucketName, objectName), nil
}
