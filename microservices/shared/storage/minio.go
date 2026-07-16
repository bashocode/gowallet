package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStorage interface {
	UploadStream(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, contentType string) (int64, error)
	EnsureBucket(ctx context.Context, bucketName string) error
}

type minioClient interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

type minioStorage struct {
	client         minioClient
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
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
		client:         client,
		maxRetries:     3,
		initialBackoff: 100 * time.Millisecond,
		maxBackoff:     2 * time.Second,
	}, nil
}

func (m *minioStorage) retry(ctx context.Context, op func() error) error {
	backoff := m.initialBackoff
	var lastErr error

	for attempt := 1; attempt <= m.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := op()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt < m.maxRetries {
			logger.Warn(ctx, "MinIO operation failed, retrying...",
				"attempt", attempt,
				"max_retries", m.maxRetries,
				"backoff", backoff.String(),
				"error", err.Error(),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > m.maxBackoff {
				backoff = m.maxBackoff
			}
		}
	}

	return fmt.Errorf("MinIO operation failed after %d attempts: %w", m.maxRetries, lastErr)
}

func (m *minioStorage) EnsureBucket(ctx context.Context, bucketName string) error {
	return m.retry(ctx, func() error {
		exists, err := m.client.BucketExists(ctx, bucketName)
		if err != nil {
			return err
		}

		if !exists {
			err = m.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (m *minioStorage) UploadStream(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, contentType string) (int64, error) {
	var size int64
	seeker, isSeeker := reader.(io.ReadSeeker)

	err := m.retry(ctx, func() error {
		if isSeeker {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("failed to seek reader to start: %w", err)
			}
		}

		info, err := m.client.PutObject(ctx, bucketName, objectName, reader, objectSize, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return err
		}

		size = info.Size
		return nil
	})

	if err != nil {
		if !isSeeker {
			logger.Warn(ctx, "MinIO UploadStream failed, and reader is not seekable so it could not be rewound for retries", "error", err.Error())
		}
		return 0, err
	}

	return size, nil
}
