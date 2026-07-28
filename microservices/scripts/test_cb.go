package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	targetURL = "http://localhost:8080/api/v1/wallets/me"
	bearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiM2U4MzQ4ZGMtZjc5YS00ZDRhLTgwYWQtMDU3NzgyYjkyZTIxIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6InVzZXIiLCJ0b2tlbl90eXBlIjoiYWNjZXNzIiwiaXNzIjoiZ293YWxsZXQtYXV0aC1zZXJ2aWNlIiwiYXVkIjpbImdvd2FsbGV0LWFwaSJdLCJleHAiOjE3ODUyNTA4NTIsIm5iZiI6MTc4NTI0OTk1MiwiaWF0IjoxNzg1MjQ5OTUyLCJqdGkiOiIzZTgzNDhkYy1mNzlhLTRkNGEtODBhZC0wNTc3ODJiOTJlMjEtYWNjZXNzLTE3ODUyNDk5NTI2OTcyNDg1MDcifQ.Gm9pzgZ220vkdwjZGIKDc7mMMFamPes7sk7Uggnd3Y8"
)

// Terminal color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

func main() {
	fmt.Println("\n==================================================================")
	fmt.Printf("  %s🚀 Circuit Breaker & Reliability Test (7 Sequential Hits)%s\n", colorCyan, colorReset)
	fmt.Println("==================================================================\n")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 1; i <= 7; i++ {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			fmt.Printf("❌ Request #%d | Failed to create request: %v\n\n", i, err)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+bearerToken)
		req.Header.Set("Content-Type", "application/json")

		startTime := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			fmt.Printf("💥 Request #%d | Network Error | Time: %dms\n", i, duration)
			fmt.Printf("   Error: %v\n\n", err)
			time.Sleep(300 * time.Millisecond)
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(bodyBytes)

		symbol := "✅"
		stateTag := fmt.Sprintf("%sSUCCESS%s", colorGreen, colorReset)

		if resp.StatusCode != http.StatusOK {
			symbol = "❌"
			stateTag = fmt.Sprintf("%sFAILED%s", colorRed, colorReset)
		}

		if strings.Contains(bodyStr, "circuit breaker is open") {
			stateTag = fmt.Sprintf("%s⚡ FAST-FAIL (Circuit Breaker OPEN)%s", colorYellow, colorReset)
		}

		fmt.Printf("%s Request #%d | Status: %d | Time: %dms | State: %s\n", symbol, i, resp.StatusCode, duration, stateTag)
		fmt.Printf("   Response: %s\n\n", bodyStr)

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("==================================================================")
	fmt.Println("  Test Execution Finished")
	fmt.Println("==================================================================\n")
}
