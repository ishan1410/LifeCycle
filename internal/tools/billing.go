package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetBillingInfo returns mock billing info for a given ticket or user.
func GetBillingInfo(ctx context.Context, ticketID string) (string, error) {
	// Mock implementation
	info := map[string]interface{}{
		"ticket_id": ticketID,
		"status":    "paid",
		"balance":   0.0,
		"plan":      "enterprise",
		"recent_charges": []map[string]interface{}{
			{"date": "2026-03-01", "amount": 100.0, "reason": "Monthly Base"},
			{"date": "2026-03-01", "amount": 100.0, "reason": "Duplicate Error"},
		},
	}
	b, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ProcessRefund processes a mock refund.
func ProcessRefund(ctx context.Context, ticketID string, amount float64) (string, error) {
	// Mock implementation
	msg := fmt.Sprintf("Successfully processing full refund of $%.2f for ticket %s.", amount, ticketID)
	return msg, nil
}
