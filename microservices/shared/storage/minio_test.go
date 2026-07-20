package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
)

type mockMinioClient struct {
	bucketExistsFn    func(ctx context.Context, bucketName string) (bool, error)
	makeBucketFn      func(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error
	putObjectFn       func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	setBucketPolicyFn func(ctx context.Context, bucketName, policy string) error

	bucketExistsCalls    int
	makeBucketCalls      int
	putObjectCalls       int
	setBucketPolicyCalls int
}

func (m *mockMinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	m.bucketExistsCalls++
	if m.bucketExistsFn != nil {
		return m.bucketExistsFn(ctx, bucketName)
	}
	return false, nil
}

func (m *mockMinioClient) SetBucketPolicy(ctx context.Context, bucketName, policy string) error {
	m.setBucketPolicyCalls++
	if m.setBucketPolicyFn != nil {
		return m.setBucketPolicyFn(ctx, bucketName, policy)
	}
	return nil
}

func (m *mockMinioClient) MakeBucket(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error {
	m.makeBucketCalls++
	if m.makeBucketFn != nil {
		return m.makeBucketFn(ctx, bucketName, options)
	}
	return nil
}

func (m *mockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	m.putObjectCalls++
	if m.putObjectFn != nil {
		return m.putObjectFn(ctx, bucketName, objectName, reader, objectSize, opts)
	}
	return minio.UploadInfo{}, nil
}

func newTestMinioStorage(mockClient minioClient) *minioStorage {
	return &minioStorage{
		client:         mockClient,
		endpoint:       "localhost:9000",
		useSSL:         false,
		maxRetries:     3,
		initialBackoff: 1 * time.Millisecond,
		maxBackoff:     5 * time.Millisecond,
	}
}

func TestEnsureBucket_Success_AlreadyExists(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFn: func(ctx context.Context, bucketName string) (bool, error) {
			return true, nil
		},
	}
	store := newTestMinioStorage(mock)

	err := store.EnsureBucket(context.Background(), "my-bucket")
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.bucketExistsCalls)
	assert.Equal(t, 0, mock.makeBucketCalls)
}

func TestEnsureBucket_Success_CreateBucket(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFn: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
	}
	store := newTestMinioStorage(mock)

	err := store.EnsureBucket(context.Background(), "my-bucket")
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.bucketExistsCalls)
	assert.Equal(t, 1, mock.makeBucketCalls)
}

func TestMakeBucketPublic_Success(t *testing.T) {
	mock := &mockMinioClient{}
	store := newTestMinioStorage(mock)

	err := store.MakeBucketPublic(context.Background(), "my-bucket")
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.setBucketPolicyCalls)
}

func TestEnsureBucket_RetryAndFail(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFn: func(ctx context.Context, bucketName string) (bool, error) {
			return false, errors.New("persistent error")
		},
	}
	store := newTestMinioStorage(mock)

	err := store.EnsureBucket(context.Background(), "my-bucket")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persistent error")
	assert.Equal(t, 3, mock.bucketExistsCalls)
}

func TestUploadStream_Success(t *testing.T) {
	mock := &mockMinioClient{
		putObjectFn: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{Size: 100}, nil
		},
	}
	store := newTestMinioStorage(mock)

	data := []byte("hello world")
	url, err := store.UploadStream(context.Background(), "my-bucket", "my-object", bytes.NewReader(data), int64(len(data)), "text/plain")
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:9000/my-bucket/my-object", url)
	assert.Equal(t, 1, mock.putObjectCalls)
}

func TestUploadStream_RetryAndSuccess_Seeker(t *testing.T) {
	attempt := 0
	mock := &mockMinioClient{
		putObjectFn: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			attempt++
			if attempt == 1 {
				// Read data to simulate partial read/exhaustion
				buf := make([]byte, 5)
				_, _ = reader.Read(buf)
				return minio.UploadInfo{}, errors.New("network timeout")
			}
			// Verify that the reader has been seeked back to start by reading and checking contents
			buf := make([]byte, 11)
			n, _ := reader.Read(buf)
			assert.Equal(t, "hello world", string(buf[:n]))
			return minio.UploadInfo{Size: 11}, nil
		},
	}
	store := newTestMinioStorage(mock)

	data := []byte("hello world")
	url, err := store.UploadStream(context.Background(), "my-bucket", "my-object", bytes.NewReader(data), int64(len(data)), "text/plain")
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:9000/my-bucket/my-object", url)
	assert.Equal(t, 2, mock.putObjectCalls)
}

func TestUploadStream_Retry_NonSeeker(t *testing.T) {
	attempt := 0
	mock := &mockMinioClient{
		putObjectFn: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			attempt++
			// Read the whole reader in the first attempt
			_, _ = io.ReadAll(reader)
			return minio.UploadInfo{}, errors.New("failure")
		},
	}
	store := newTestMinioStorage(mock)

	// bytes.Buffer does not implement io.ReadSeeker
	var buf bytes.Buffer
	buf.Write([]byte("hello world"))

	_, err := store.UploadStream(context.Background(), "my-bucket", "my-object", &buf, 11, "text/plain")
	assert.Error(t, err)
	assert.Equal(t, 3, mock.putObjectCalls)
}

func TestEnsureBucket_ContextCancelled(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFn: func(ctx context.Context, bucketName string) (bool, error) {
			return false, errors.New("some error")
		},
	}
	store := newTestMinioStorage(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel context immediately

	err := store.EnsureBucket(ctx, "my-bucket")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestUploadStream_Success_PublicURL(t *testing.T) {
	mock := &mockMinioClient{
		putObjectFn: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{Size: 100}, nil
		},
	}
	store := &minioStorage{
		client:         mock,
		endpoint:       "minio:9000",
		publicURL:      "https://cdn.example.com",
		useSSL:         false,
		maxRetries:     3,
		initialBackoff: 1 * time.Millisecond,
		maxBackoff:     5 * time.Millisecond,
	}

	data := []byte("hello world")
	url, err := store.UploadStream(context.Background(), "my-bucket", "my-object", bytes.NewReader(data), int64(len(data)), "text/plain")
	assert.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/my-bucket/my-object", url)
}

