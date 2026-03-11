package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
	"github.com/ishanpatel/multi-agent-orchestrator/pkg/llm"
	"github.com/tmc/langchaingo/llms"
)

// SupervisorAgent evaluates the TicketState and decides the next routing step.
type SupervisorAgent struct {
	llmClient *llm.Client
}

// NewSupervisorAgent creates a new SupervisorAgent.
func NewSupervisorAgent(client *llm.Client) *SupervisorAgent {
	return &SupervisorAgent{
		llmClient: client,
	}
}

type SupervisorDecision struct {
	Route  string `json:"route"` // "ECHO", "REMINDER", "MODIFY_REMINDER", "RESOLVED", "NEEDS_MORE_INFO"
	Reason string `json:"reason"`
}

// Execute analyzes the conversation and updates the status.
func (a *SupervisorAgent) Execute(ctx context.Context, ticket *state.TicketState) error {
	slog.Info("Supervisor Agent analyzing ticket", "ticket_id", ticket.TicketID)

	prompt := `You are the Supervisor Agent for an enterprise support system.
Your job is to read the conversation history and decide the next step.
Available routes:
- REMINDER: If the user explicitly asks to CREATE a new reminder, alert, or notification about something in the future.
- MODIFY_REMINDER: If the user explicitly asks to CANCEL, CHANGE, or MODIFY an existing reminder they previously set.
- ECHO: If the user is just saying hello, asking a general question, or anything that doesn't fit the above.
- RESOLVED: If all issues have been completely answered by previous messages.
- NEEDS_MORE_INFO: If the user's request is too vague to route, or if you need clarification.

IMPORTANT: If the user has MULTIPLE distinct issues (e.g., a billing issue AND a technical issue), pick ONLY ONE unresolved issue to route first. Once that agent completes its task, the ticket will come back to you, and you can route the remaining unresolved issue to the next department. Do not say NEEDS_MORE_INFO just because there are multiple issues.

Analyze the conversation and return ONLY a JSON object:
{"route": "...", "reason": "..."}
DO NOT include Markdown formatting (like ` + "```json" + `), just the raw JSON.

Conversation History:
`

	// Build the context
	for _, msg := range ticket.ConversationHistory {
		prompt += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	resp, err := a.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return fmt.Errorf("supervisor generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from supervisor")
	}

	rawResp := strings.TrimSpace(resp.Choices[0].Content)
	// clean up potential markdown block if gemini ignores the instruction
	rawResp = strings.TrimPrefix(rawResp, "```json")
	rawResp = strings.TrimPrefix(rawResp, "```")
	rawResp = strings.TrimSuffix(rawResp, "```")
	rawResp = strings.TrimSpace(rawResp)

	var decision SupervisorDecision
	if err := json.Unmarshal([]byte(rawResp), &decision); err != nil {
		return fmt.Errorf("failed to parse supervisor decision: %w\nRaw: %s", err, rawResp)
	}

	slog.Info("Supervisor decision", "route", decision.Route, "reason", decision.Reason)
	// 3. Status Transition based on the decision
	switch decision.Route {
	case "REMINDER":
		ticket.UpdateStatus(state.StatusRoutedReminder)
	case "MODIFY_REMINDER":
		ticket.UpdateStatus(state.StatusRoutedModifyReminder)
	case "ECHO":
		ticket.UpdateStatus(state.StatusRoutedEcho)
	case "RESOLVED":
		ticket.UpdateStatus(state.StatusResolved)
	case "NEEDS_MORE_INFO":
		ticket.UpdateStatus(state.StatusNeedsMoreInfo)
		// We can add a message from the system/assistant to ask the user
		ticket.AddMessage("assistant", decision.Reason)
	default:
		slog.Warn("Unknown route, sending to MORE_INFO", "route", decision.Route)
		ticket.UpdateStatus(state.StatusNeedsMoreInfo)
	}

	return nil
}
