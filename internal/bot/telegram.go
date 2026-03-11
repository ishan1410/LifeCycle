package bot

import (
	"context"
	"fmt"
	"log/slog"
	
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/agents"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
)

// TelegramBot handles the interaction with the Telegram API.
type TelegramBot struct {
	api   *tgbotapi.BotAPI
	graph *agents.GraphOrchestrator
}

// NewTelegramBot initializes a new Telegram bot instance.
func NewTelegramBot(token string, graph *agents.GraphOrchestrator) (*TelegramBot, error) {
	botApi, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	slog.Info("Authorized on account", "username", botApi.Self.UserName)

	return &TelegramBot{
		api:   botApi,
		graph: graph,
	}, nil
}

// StartPolling begins long-polling for updates from Telegram (Ideal for local dev).
func (b *TelegramBot) StartPolling(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	slog.Info("Started Telegram Long Polling")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Telegram bot polling")
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case update := <-updates:
			// Ignore any non-Message updates
			if update.Message == nil {
				continue
			}

			slog.Info("Received message", "user", update.Message.From.UserName, "text", update.Message.Text)

			// Process the message asynchronously so we don't block the polling loop
			go b.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage takes an incoming Telegram message and feeds it to the Orchestrator.
func (b *TelegramBot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatIDStr := fmt.Sprintf("%d", msg.Chat.ID)
	
	// Create a new TicketState, using the Chat ID as the Ticket ID
	ticket := state.NewTicketState(chatIDStr, msg.Text)

	// Execute the Orchestrator Graph
	if err := b.graph.Run(ctx, ticket); err != nil {
		slog.Error("Graph execution failed", "error", err, "chat_id", chatIDStr)
		b.SendMessage(msg.Chat.ID, "I'm sorry, I encountered an internal error while processing your request.")
		return
	}

	// The Orchestrator has finished, its response should be in ResolutionNotes
	// Wait, if it's NEEDS_MORE_INFO, the assistant message is in the history or ResolutionNotes
	// We'll use the last assistant message, or ResolutionNotes.
	
	var finalResponse string
	if ticket.ResolutionNotes != "" {
		finalResponse = ticket.ResolutionNotes
	} else {
		// Fallback to the last assistant message
		for i := len(ticket.ConversationHistory) - 1; i >= 0; i-- {
			if ticket.ConversationHistory[i].Role == "assistant" {
				finalResponse = ticket.ConversationHistory[i].Content
				break
			}
		}
	}

	if finalResponse == "" {
		finalResponse = "Your request was processed, but I have no notes to show."
	}

	b.SendMessage(msg.Chat.ID, finalResponse)
}

// SendMessage allows any part of the application to proactively send a message to a user.
func (b *TelegramBot) SendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		slog.Error("Failed to send telegram reply", "error", err, "chat_id", chatID)
	}
}
