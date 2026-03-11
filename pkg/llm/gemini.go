package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

// Client wraps the langchaingo Google AI client to implement retries and timeouts.
type Client struct {
	model *googleai.GoogleAI
}

// NewClient initializes a new Gemini 1.5 Flash client.
func NewClient(ctx context.Context) (*Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable is missing")
	}

	model, err := googleai.New(
		ctx,
		googleai.WithAPIKey(apiKey),
		googleai.WithDefaultModel("gemini-1.5-flash-latest"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Google AI model: %w", err)
	}

	return &Client{
		model: model,
	}, nil
}

// GenerateContent generates a response using Gemini 1.5 Flash.
// It wraps the call with a timeout and a simple retry logic to handle 429 rate limits.
func (c *Client) GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	maxRetries := 2
	baseWait := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Enforce a hard timeout for each API attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, err := c.model.GenerateContent(attemptCtx, messages, opts...)
		cancel()

		if err == nil {
			return resp, nil
		}

		slog.Warn("LLM generation attempt failed", "attempt", i+1, "error", err)

		// Basic retry on typical transit/timeout errors, waiting incrementally longer
		if i < maxRetries-1 {
			time.Sleep(baseWait * time.Duration(i+1))
			continue
		}

		return nil, fmt.Errorf("generate content failed after %d retries: %w", maxRetries, err)
	}

	return nil, errors.New("unexpected error in retry loop")
}

// Model returns the underlying langchaingo googleai instance
// This is useful if we need to pass it into direct langchaingo agent abstractions.
func (c *Client) Model() *googleai.GoogleAI {
	return c.model
}
