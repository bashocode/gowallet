package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/model"
	"github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pbTransaction "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
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
	paymentRepo         repository.PaymentRepository
	stripeSecretKey     string
	stripeWebhookSecret string
	baseURL             string
	txClient            pbTransaction.TransactionServiceClient
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	stripeSecretKey string,
	stripeWebhookSecret string,
	txClient pbTransaction.TransactionServiceClient,
	baseURL string,
) PaymentService {
	// Initialize stripe key globally
	stripe.Key = stripeSecretKey

	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &paymentService{
		paymentRepo:         paymentRepo,
		stripeSecretKey:     stripeSecretKey,
		stripeWebhookSecret: stripeWebhookSecret,
		baseURL:             baseURL,
		txClient:            txClient,
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
	var event stripe.Event
	var err error

	// Verify webhook signature if secret is provided
	if s.stripeWebhookSecret != "" {
		event, err = stripeWebhook.ConstructEventWithOptions(payload, sigHeader, s.stripeWebhookSecret, stripeWebhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
		if err != nil {
			logger.Log.Warn("Stripe webhook signature verification failed", slog.Any("error", err))
			return fmt.Errorf("bad webhook signature: %w", err)
		}
	} else {
		// Fallback for development without configured webhook secret
		if err := json.Unmarshal(payload, &event); err != nil {
			logger.Log.Error("Failed to parse Stripe webhook event json", slog.Any("error", err))
			return fmt.Errorf("bad webhook JSON: %w", err)
		}
	}

	logger.Log.Info("Received Stripe webhook event", "type", event.Type, "id", event.ID)

	if event.Type == "checkout.session.completed" {
		var sess stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &sess)
		if err != nil {
			logger.Log.Error("Failed to parse checkout session data", slog.Any("error", err))
			return err
		}

		logger.Log.Info("Stripe checkout completed", "session_id", sess.ID, "payment_status", sess.PaymentStatus)

		// 1. Get payment from database
		p, err := s.paymentRepo.GetByStripeSessionID(ctx, sess.ID)
		if err != nil {
			logger.Log.Error("Failed to fetch payment record", "session_id", sess.ID, slog.Any("error", err))
			return err
		}
		if p == nil {
			logger.Log.Warn("Payment record not found for Stripe session", "session_id", sess.ID)
			return fmt.Errorf("payment session not found: %s", sess.ID)
		}

		// 2. If already completed, skip processing (idempotency)
		if p.Status == "completed" {
			logger.Log.Info("Payment already completed, skipping", "session_id", sess.ID)
			return nil
		}

		// 3. Update payment status to completed in database
		if err := s.paymentRepo.UpdateStatus(ctx, sess.ID, "completed"); err != nil {
			logger.Log.Error("Failed to update payment status to completed", "session_id", sess.ID, slog.Any("error", err))
			return err
		}

		// 4. Trigger wallet credit via gRPC to transaction-service
		_, err = s.txClient.TopUp(ctx, &pbTransaction.TopUpRequest{
			UserId:         p.UserID,
			Amount:         p.Amount.String(),
			IdempotencyKey: sess.ID, // Stripe session ID as idempotency key
		})
		if err != nil {
			logger.Log.Error("gRPC TopUp failed", "user_id", p.UserID, "amount", p.Amount, slog.Any("error", err))
			// Rollback DB status to pending so it can be retried later
			_ = s.paymentRepo.UpdateStatus(ctx, sess.ID, "pending")
			return fmt.Errorf("gRPC TopUp failed: %w", err)
		}

		logger.Log.Info("Wallet successfully credited via gRPC", "session_id", sess.ID, "user_id", p.UserID, "amount", p.Amount)
	}

	return nil
}
