package model

// Add these to the end of tx.go
type WalletInquiry struct {
	Valid     bool   `json:"valid"`
	AccountID string `json:"account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type ExternalInquiryRequest struct {
	Email string `json:"email" binding:"required,email" example:"recipient@example.com"`
}
