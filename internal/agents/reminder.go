package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/db"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
	"github.com/ishanpatel/multi-agent-orchestrator/pkg/llm"
	"github.com/tmc/langchaingo/llms"
)

// ReminderAgent extracts scheduling instructions and queues a background goroutine.
type ReminderAgent struct {
	llmClient *llm.Client
	db        *db.Database
}

func NewReminderAgent(client *llm.Client, database *db.Database) *ReminderAgent {
	return &ReminderAgent{
		llmClient: client,
		db:        database,
	}
}

type ReminderData struct {
	TargetTime   string `json:"target_time"`
	ReminderText string `json:"reminder_text"`
}

// Execute uses the LLM to parse out *when* and *what* to remind the user about.
func (a *ReminderAgent) Execute(ctx context.Context, ticket *state.TicketState) error {
	slog.Info("Reminder Agent analyzing ticket", "ticket_id", ticket.TicketID)

	// Convert TicketID (which is ChatID) to int64
	var chatID int64
	fmt.Sscanf(ticket.TicketID, "%d", &chatID)

	prompt := fmt.Sprintf(`You are the Reminder Scheduler Agent. 
The user wants to be reminded or alerted about something in the future.
The current system time (UTC) is: %s

Read the user's request and figure out exactly WHEN they want to be reminded.
Return the exact target Date and Time formatted strictly as an ISO-8601 string, and the text of the reminder.

For example, if current time is "2026-10-15T15:00:00Z" and the user says "remind me in 5 minutes to stretch", the target time is "2026-10-15T15:05:00Z".

Return ONLY a JSON object:
{"target_time": "2026-10-15T15:05:00Z", "reminder_text": "Time to stretch!"}

DO NOT include Markdown formatting (like `+"```json"+`), just the raw JSON.`, time.Now().UTC().Format(time.RFC3339))

	for _, msg := range ticket.ConversationHistory {
		if msg.Role == "user" {
			prompt += fmt.Sprintf("\nUser Request: %s", msg.Content)
		}
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	resp, err := a.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return fmt.Errorf("reminder generation failed: %w", err)
	}

	rawResp := strings.TrimSpace(resp.Choices[0].Content)
	rawResp = strings.TrimPrefix(rawResp, "```json")
	rawResp = strings.TrimPrefix(rawResp, "```")
	rawResp = strings.TrimSuffix(rawResp, "```")
	rawResp = strings.TrimSpace(rawResp)

	var data ReminderData
	if err := json.Unmarshal([]byte(rawResp), &data); err != nil {
		return fmt.Errorf("failed to parse reminder scheduling: %w\nRaw: %s", err, rawResp)
	}

	slog.Info("Scheduling Reminder", "target_time", data.TargetTime, "text", data.ReminderText, "chat_id", chatID)

	targetTime, err := time.Parse(time.RFC3339, data.TargetTime)
	if err != nil {
		return fmt.Errorf("failed to parse ISO8601 target time %s: %w", data.TargetTime, err)
	}

	// Persist the job with the specific ChatID
	jobID, err := a.db.SaveReminder(chatID, targetTime, data.ReminderText)
	if err != nil {
		return fmt.Errorf("failed to save reminder to DB: %w", err)
	}

	// Calculate relative time for a clearer response (since we don't know user timezone)
	duration := time.Until(targetTime)
	relativeDesc := ""
	if duration > 0 {
		relativeDesc = fmt.Sprintf(" (in %s)", duration.Round(time.Second))
	}

	// Load PDT (America/Los_Angeles) for display
	loc, _ := time.LoadLocation("America/Los_Angeles")
	displayTime := targetTime.In(loc).Format("Jan 02, 3:04 PM")

	responseMsg := fmt.Sprintf("I've scheduled a reminder for %s PT%s. (Job ID: %d)", 
		displayTime, relativeDesc, jobID)

	ticket.ResolutionNotes = responseMsg
	ticket.UpdateStatus(state.StatusResolved)
	ticket.AddMessage("assistant", responseMsg)

	return nil
}
