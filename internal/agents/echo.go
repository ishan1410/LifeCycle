package agents

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
)

// EchoAgent acts as a simple worker that just echoes back the user's latest message.
type EchoAgent struct{}

func NewEchoAgent() *EchoAgent {
	return &EchoAgent{}
}

// Execute performs the echo logic.
func (a *EchoAgent) Execute(ctx context.Context, ticket *state.TicketState) error {
	slog.Info("Echo Agent executing", "ticket_id", ticket.TicketID)

	// Get latest user message
	var lastMsg string
	for i := len(ticket.ConversationHistory) - 1; i >= 0; i-- {
		if ticket.ConversationHistory[i].Role == "user" {
			lastMsg = ticket.ConversationHistory[i].Content
			break
		}
	}

	echoText := fmt.Sprintf("Echo Agent received: %s", lastMsg)
	slog.Info("Echo Agent action", "echoText", echoText)

	// Set the resolution notes to the echo response
	ticket.ResolutionNotes = echoText

	// Tell the orchestrator we are done resolving this task
	ticket.UpdateStatus(state.StatusResolved)
	
	// Add the assistant response to the conversation history
	ticket.AddMessage("assistant", echoText)

	return nil
}
