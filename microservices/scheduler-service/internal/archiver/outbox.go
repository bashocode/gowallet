package archiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/storage"
)

type ArchivableEvent struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type OutboxClient interface {
	FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]ArchivableEvent, error)
	DeleteArchivedEvents(ctx context.Context, ids []string) error
}

type OutboxArchiver struct {
	name       string
	bucketName string
	client     OutboxClient
	minio      storage.ObjectStorage
	minAge     time.Duration
	interval   time.Duration
}

func NewOutboxArchiver(name string, bucketName string, client OutboxClient, minio storage.ObjectStorage, minAge time.Duration, interval time.Duration) *OutboxArchiver {
	return &OutboxArchiver{
		name:       name,
		bucketName: bucketName,
		client:     client,
		minio:      minio,
		minAge:     minAge,
		interval:   interval,
	}
}

func (a *OutboxArchiver) Start(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	logger.Info(ctx, "Starting Outbox Archiver background worker", "name", a.name)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.archiveBatch(ctx)
		}
	}
}

func (a *OutboxArchiver) archiveBatch(ctx context.Context) {
	events, err := a.client.FetchEventsToArchive(ctx, a.minAge, 500)
	if err != nil || len(events) == 0 {
		return
	}

	data, err := json.Marshal(events)
	if err != nil {
		logger.Error(ctx, "Failed to serialize outbox archive", "name", a.name, "error", err.Error())
		return
	}

	now := time.Now()
	objectName := fmt.Sprintf("%s/year=%d/month=%02d/day=%02d/outbox-%d.json", a.name, now.Year(), now.Month(), now.Day(), now.UnixNano())

	reader := bytes.NewReader(data)
	_, err = a.minio.UploadStream(ctx, a.bucketName, objectName, reader, int64(len(data)), "application/json")
	if err != nil {
		logger.Error(ctx, "Failed to upload outbox archive to MinIO", "name", a.name, "bucket", a.bucketName, "error", err.Error())
		return
	}

	var ids []string
	for _, ev := range events {
		ids = append(ids, ev.ID)
	}

	err = a.client.DeleteArchivedEvents(ctx, ids)
	if err != nil {
		logger.Error(ctx, "Failed to delete archived outbox rows from database", "name", a.name, "error", err.Error())
		return
	}

	logger.Info(ctx, "Successfully archived outbox batch to MinIO object storage", "name", a.name, "archived_events_count", len(events))
}
