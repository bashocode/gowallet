package archiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/storage"
	pb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
)

type OutboxArchiver struct {
	txClient pb.TransactionServiceClient
	minio    storage.ObjectStorage
	minAge   time.Duration
	interval time.Duration
}

func NewOutboxArchiver(txClient pb.TransactionServiceClient, minio storage.ObjectStorage, minAge time.Duration, interval time.Duration) *OutboxArchiver {
	return &OutboxArchiver{
		txClient: txClient,
		minio:    minio,
		minAge:   minAge,
		interval: interval,
	}
}

func (a *OutboxArchiver) Start(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	logger.Info(ctx, "Starting Outbox Archiver background worker")

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
	resp, err := a.txClient.FetchEventsToArchive(ctx, &pb.FetchEventsToArchiveRequest{
		MinAgeSeconds: int64(a.minAge.Seconds()),
		Limit:         500,
	})
	if err != nil || len(resp.Events) == 0 {
		return
	}

	data, err := json.Marshal(resp.Events)
	if err != nil {
		logger.Error(ctx, "Failed to serialize outbox archive", "error", err.Error())
		return
	}

	now := time.Now()
	objectName := fmt.Sprintf("year=%d/month=%02d/day=%02d/outbox-%d.json", now.Year(), now.Month(), now.Day(), now.UnixNano())

	reader := bytes.NewReader(data)
	_, err = a.minio.UploadStream(ctx, "outbox-archives", objectName, reader, int64(len(data)), "application/json")
	if err != nil {
		logger.Error(ctx, "Failed to upload outbox archive to MinIO", "error", err.Error())
		return
	}

	var ids []string
	for _, ev := range resp.Events {
		ids = append(ids, ev.Id)
	}

	deleteResp, err := a.txClient.DeleteArchivedEvents(ctx, &pb.DeleteArchivedEventsRequest{Ids: ids})
	if err != nil || !deleteResp.Success {
		logger.Error(ctx, "Failed to delete archived outbox rows from database", "error", err.Error())
		return
	}

	logger.Info(ctx, "Successfully archived outbox batch to MinIO object storage", "archived_events_count", len(resp.Events))
}
