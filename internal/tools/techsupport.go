package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// CheckServerStatus returns the mock status of a server.
func CheckServerStatus(ctx context.Context, serverID string) (string, error) {
	// Mock implementation
	status := map[string]interface{}{
		"server_id": serverID,
		"status":    "down", // Simulating the down server
		"uptime":    "0s",
		"last_error": "network interface crash detected",
	}
	b, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RestartServer restarts a specified server.
func RestartServer(ctx context.Context, serverID string) (string, error) {
	// Mock implementation
	msg := fmt.Sprintf("Server %s restarted successfully and is now running.", serverID)
	return msg, nil
}
