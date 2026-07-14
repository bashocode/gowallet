package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/publisher"
	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v78"
	checkoutSession "github.com/stripe/stripe-go/v78/checkout/session"
	stripeWebhook "github.com/stripe/stripe-go/v78/webhook"
)

type PaymentService interface {
	CreateCheckoutSession(ctx context.Context, userID string, req model.StripeCheckoutRequest) (*model.StripeCheckoutResponse, error)
	ProcessWebhook(ctx context.Context, payload []byte, sigHeader string) error
}

type paymentService struct {
	db                  *sql.DB
	paymentRepo         repository.PaymentRepository
	outboxRepo          repository.OutboxRepository
	stripeSecretKey     string
	stripeWebhookSecret string
	baseURL             string
	publisher           publisher.PaymentPublisher
}

func NewPaymentService(
	db *sql.DB,
	paymentRepo repository.PaymentRepository,
	outboxRepo repository.OutboxRepository,
	stripeSecretKey string,
	stripeWebhookSecret string,
	pub publisher.PaymentPublisher,
	baseURL string,
) PaymentService {
	// Initialize stripe key globally
	stripe.Key = stripeSecretKey

	return &paymentService{
		db:                  db,
		paymentRepo:         paymentRepo,
		outboxRepo:          outboxRepo,
		stripeSecretKey:     stripeSecretKey,
		stripeWebhookSecret: stripeWebhookSecret,
		baseURL:             baseURL,
		publisher:           pub,
	}
}

func (s *paymentService) CreateCheckoutSession(ctx context.Context, userID string, req model.StripeCheckoutRequest) (*model.StripeCheckoutResponse, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("amount must be greater than zero")
	}

	currency := strings.ToLower(req.Currency)
	if currency == "" {
		currency = "usd"
	}

	// Stripe accepts amount in cents (e.g. $10.00 = 1000 cents)
	cents := req.Amount.Mul(decimal.NewFromInt(100)).IntPart()

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("GoWallet Top-Up"),
						Description: stripe.String(fmt.Sprintf("Top-up balance for User ID: %s", userID)),
					},
					UnitAmount: stripe.Int64(cents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.baseURL + "/api/v1/payments/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.baseURL + "/api/v1/payments/cancel"),
	}

	sess, err := checkoutSession.New(params)
	if err != nil {
		logger.Log.Error("Failed to create Stripe Checkout Session", slog.Any("error", err))
		return nil, err
	}

	// Save pending payment record to database
	paymentRecord := &model.Payment{
		ID:              uuid.New().String(),
		UserID:          userID,
		Amount:          req.Amount,
		Currency:        currency,
		StripeSessionID: sess.ID,
		Status:          "pending",
	}

	if err := s.paymentRepo.Create(ctx, paymentRecord); err != nil {
		logger.Log.Error("Failed to save payment record", slog.Any("error", err))
		return nil, err
	}

	return &model.StripeCheckoutResponse{
		CheckoutURL: sess.URL,
		SessionID:   sess.ID,
	}, nil
}

func (s *paymentService) ProcessWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	var stripeEvent stripe.Event
	var err error

	// Verify webhook signature if secret is provided
	if s.stripeWebhookSecret != "" {
		stripeEvent, err = stripeWebhook.ConstructEventWithOptions(payload, sigHeader, s.stripeWebhookSecret, stripeWebhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
		if err != nil {
			logger.Log.Warn("Stripe webhook signature verification failed", slog.Any("error", err))
			return fmt.Errorf("bad webhook signature: %w", err)
		}
	} else {
		// Fallback for development without configured webhook secret
		if err := json.Unmarshal(payload, &stripeEvent); err != nil {
			logger.Log.Error("Failed to parse Stripe webhook event json", slog.Any("error", err))
			return fmt.Errorf("bad webhook JSON: %w", err)
		}
	}

	logger.Log.Info("Received Stripe webhook event", "type", stripeEvent.Type, "id", stripeEvent.ID)

	if stripeEvent.Type == "checkout.session.completed" {
		var sess stripe.CheckoutSession
		err := json.Unmarshal(stripeEvent.Data.Raw, &sess)
		if err != nil {
			logger.Log.Error("Failed to parse checkout session data", slog.Any("error", err))
			return err
		}

		logger.Log.Info("Stripe checkout completed", "session_id", sess.ID, "payment_status", sess.PaymentStatus)

		// Use transactional outbox pattern
		if err := s.markPaymentSettledTx(ctx, sess.ID); err != nil {
			logger.Log.Error("Failed to process payment settlement", "session_id", sess.ID, slog.Any("error", err))
			return err
		}

		logger.Log.Info("Payment settlement recorded in outbox", "session_id", sess.ID)
	}

	return nil
}

func (s *paymentService) markPaymentSettledTx(ctx context.Context, stripeSessionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get payment with transaction
	p, err := s.paymentRepo.GetByStripeSessionIDTx(ctx, tx, stripeSessionID)
	if err != nil {
		return fmt.Errorf("failed to fetch payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found: %s", stripeSessionID)
	}

	// Idempotency check
	if p.Status == "completed" {
		logger.Log.Info("Payment already completed, skipping", "session_id", stripeSessionID)
		return tx.Commit()
	}

	// Update payment status
	if err := s.paymentRepo.UpdateStatusTx(ctx, tx, stripeSessionID, "completed"); err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	// Create payment event
	event := model.PaymentSettledEvent{
		EventID:           uuid.NewString(),
		EventType:         "payment.settled",
		Provider:          "stripe",
		ProviderPaymentID: stripeSessionID,
		PaymentID:         p.ID,
		UserID:            p.UserID,
		Amount:            p.Amount.String(),
		Currency:          p.Currency,
		Status:            "success",
		OccurredAt:        time.Now().UTC(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create outbox event
	outboxEvent := &model.OutboxEvent{
		ID:          event.EventID,
		EventType:   event.EventType,
		AggregateID: p.ID,
		Payload:     payload,
		Status:      "pending",
		Attempts:    0,
	}

	if err := s.outboxRepo.CreateTx(ctx, tx, outboxEvent); err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return tx.Commit()
}
