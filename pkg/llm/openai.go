package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// Client wraps the langchaingo OpenAI client to implement retries and timeouts.
type Client struct {
	model *openai.LLM
}

// NewClient initializes a new OpenAI GPT-4o-mini client.
func NewClient(ctx context.Context) (*Client, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable is missing")
	}

	model, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel("gpt-4o-mini"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI model: %w", err)
	}

	slog.Info("LLM Client initialized", "model", "gpt-4o-mini")

	return &Client{
		model: model,
	}, nil
}

// GenerateContent generates a response using OpenAI.
// It wraps the call with a timeout and a simple retry logic.
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

// Model returns the underlying langchaingo openai instance
func (c *Client) Model() *openai.LLM {
	return c.model
}
