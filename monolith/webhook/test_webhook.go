package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	// Build payload
	payload := map[string]interface{}{
		"transfer_id":     "4f3e8ea1-25a0-4124-878f-ed2917763165",
		"status":          "success",
		"receiver_email":  "rimuru@example.com",
		"amount":          "4000",
		"idempotency_key": "test-transfer-004",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("failed to marshal payload:", err)
	}

	// Generate HMAC signature
	h := hmac.New(sha256.New, []byte("gowallet-webhook-secret-change-me"))
	h.Write(payloadBytes)
	signature := hex.EncodeToString(h.Sum(nil))

	// Print payload
	fmt.Println("=== PAYLOAD ===")
	fmt.Println(string(payloadBytes))
	fmt.Println()

	// Print signature
	fmt.Println("=== SIGNATURE ===")
	fmt.Println(signature)
	fmt.Println()

	// Print curl command
	fmt.Println("=== CURL COMMAND ===")
	fmt.Printf("curl -X POST http://localhost:8086/api/v1/transactions/transfers/webhook \\\n")
	fmt.Printf("  -H \"X-API-Key: %s\" \\\n", "gowallet-webhook-secret-change-me")
	fmt.Printf("  -H \"X-Webhook-Signature: %s\" \\\n", signature)
	fmt.Printf("  -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("  -d '%s'\n", string(payloadBytes))
}
