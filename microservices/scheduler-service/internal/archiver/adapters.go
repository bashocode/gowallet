package archiver

import (
	"context"
	"fmt"
	"time"

	paymentPb "github.com/bashocode/gowallet/microservices/payment-service/proto/payment"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	userPb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
)

type TransactionOutboxAdapter struct {
	Client txPb.TransactionServiceClient
}

func (a *TransactionOutboxAdapter) FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]ArchivableEvent, error) {
	resp, err := a.Client.FetchEventsToArchive(ctx, &txPb.FetchEventsToArchiveRequest{
		MinAgeSeconds: int64(minAge.Seconds()),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}
	events := make([]ArchivableEvent, len(resp.Events))
	for i, ev := range resp.Events {
		events[i] = ArchivableEvent{
			ID:        ev.Id,
			EventType: ev.EventType,
			Payload:   ev.Payload,
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt,
		}
	}
	return events, nil
}

func (a *TransactionOutboxAdapter) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	resp, err := a.Client.DeleteArchivedEvents(ctx, &txPb.DeleteArchivedEventsRequest{Ids: ids})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("failed to delete archived events: %s", resp.Error)
	}
	return nil
}

type UserOutboxAdapter struct {
	Client userPb.UserServiceClient
}

func (a *UserOutboxAdapter) FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]ArchivableEvent, error) {
	resp, err := a.Client.FetchEventsToArchive(ctx, &userPb.FetchEventsToArchiveRequest{
		MinAgeSeconds: int64(minAge.Seconds()),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}
	events := make([]ArchivableEvent, len(resp.Events))
	for i, ev := range resp.Events {
		events[i] = ArchivableEvent{
			ID:        ev.Id,
			EventType: ev.EventType,
			Payload:   ev.Payload,
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt,
		}
	}
	return events, nil
}

func (a *UserOutboxAdapter) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	resp, err := a.Client.DeleteArchivedEvents(ctx, &userPb.DeleteArchivedEventsRequest{Ids: ids})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("failed to delete archived events: %s", resp.Error)
	}
	return nil
}

type PaymentOutboxAdapter struct {
	Client paymentPb.PaymentServiceClient
}

func (a *PaymentOutboxAdapter) FetchEventsToArchive(ctx context.Context, minAge time.Duration, limit int) ([]ArchivableEvent, error) {
	resp, err := a.Client.FetchEventsToArchive(ctx, &paymentPb.FetchEventsToArchiveRequest{
		MinAgeSeconds: int64(minAge.Seconds()),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}
	events := make([]ArchivableEvent, len(resp.Events))
	for i, ev := range resp.Events {
		events[i] = ArchivableEvent{
			ID:        ev.Id,
			EventType: ev.EventType,
			Payload:   ev.Payload,
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt,
		}
	}
	return events, nil
}

func (a *PaymentOutboxAdapter) DeleteArchivedEvents(ctx context.Context, ids []string) error {
	resp, err := a.Client.DeleteArchivedEvents(ctx, &paymentPb.DeleteArchivedEventsRequest{Ids: ids})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("failed to delete archived events: %s", resp.Error)
	}
	return nil
}
