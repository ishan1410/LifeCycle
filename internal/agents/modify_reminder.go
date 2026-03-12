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

// ModifyReminderAgent searches the DB and updates or cancels an ongoing alarm.
type ModifyReminderAgent struct {
	llmClient *llm.Client
	db        *db.Database
}

func NewModifyReminderAgent(client *llm.Client, database *db.Database) *ModifyReminderAgent {
	return &ModifyReminderAgent{
		llmClient: client,
		db:        database,
	}
}

type ModifyData struct {
	JobID      int    `json:"job_id"`      // The specific ID of the reminder they want to modify (or 0 if not found)
	TargetTime string `json:"target_time"` // The newly requested time (or empty if cancelling)
	IsCancel   bool   `json:"is_cancel"`   // True if the user wants to delete the alarm
}
func (a *ModifyReminderAgent) Execute(ctx context.Context, ticket *state.TicketState) error {
	slog.Info("Modify Reminder Agent analyzing ticket", "ticket_id", ticket.TicketID)

	// Convert TicketID (which is ChatID) to int64
	var chatID int64
	fmt.Sscanf(ticket.TicketID, "%d", &chatID)

	// 1. Fetch only this user's pending reminders for privacy.
	jobs, err := a.db.GetPendingRemindersByChatID(chatID)
	if err != nil {
		return fmt.Errorf("failed to fetch pending reminders: %w", err)
	}

	if len(jobs) == 0 {
		msg := "You don't have any pending reminders scheduled to modify."
		ticket.ResolutionNotes = msg
		ticket.UpdateStatus(state.StatusResolved)
		ticket.AddMessage("assistant", msg)
		return nil
	}

	// 2. Build the context of existing jobs for the prompt
	jobsContext := "Active Reminders List:\n"
	for _, j := range jobs {
		jobsContext += fmt.Sprintf("- [Job ID: %d] Target Time: %s | Text: '%s'\n", j.ID, j.TargetTime.Format("Jan 02, 3:04 PM"), j.ReminderText)
	}

	prompt := fmt.Sprintf(`You are the Reminder Modifier Agent. 
The user previously set a reminder and now wants to change the time or cancel it entirely.

The current system time is: %s

%s

Read the request and figure out WHICH Job ID the user is referring to from the active reminders list above, and what they want to do with it.
Extract:
1. Job ID: The exactly matching integer ID from the list above. (Return 0 if it's completely unclear).
2. Is Cancel: set to true if they are deleting/canceling the alarm.
3. Target Time: The strictly formatted ISO-8601 target time if they are updating the time. If they are canceling, leave this empty.

For example, if they say "Change my workout reminder to 8 PM tonight":
{"job_id": 1, "target_time": "2026-10-15T20:00:00Z", "is_cancel": false}

If they say "Cancel my reminder to drink water":
{"job_id": 2, "target_time": "", "is_cancel": true}

Return ONLY a JSON object. DO NOT include Markdown formatting.`, time.Now().UTC().Format(time.RFC3339), jobsContext)

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
		return fmt.Errorf("reminder modification generation failed: %w", err)
	}

	rawResp := strings.TrimSpace(resp.Choices[0].Content)
	rawResp = strings.TrimPrefix(rawResp, "```json")
	rawResp = strings.TrimPrefix(rawResp, "```")
	rawResp = strings.TrimSuffix(rawResp, "```")
	rawResp = strings.TrimSpace(rawResp)

	var data ModifyData
	if err := json.Unmarshal([]byte(rawResp), &data); err != nil {
		return fmt.Errorf("failed to parse reminder modification: %w\nRaw: %s", err, rawResp)
	}

	if data.JobID == 0 {
		msg := "I'm sorry, I couldn't figure out which reminder you wanted to modify based on your message."
		ticket.ResolutionNotes = msg
		ticket.UpdateStatus(state.StatusResolved)
		ticket.AddMessage("assistant", msg)
		return nil
	}

	// 3. Find the exact job they targeted to get its text for the response
	var targetJob db.ReminderJob
	found := false
	for _, j := range jobs {
		if j.ID == data.JobID {
			targetJob = j
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("LLM returned job_id %d which is not in the pending list", data.JobID)
	}

	var responseMsg string
	if data.IsCancel {
		// 4a. Cancel the job
		err = a.db.MarkCancelled(targetJob.ID)
		responseMsg = fmt.Sprintf("I have cancelled your reminder: '%s'", targetJob.ReminderText)
	} else {
		// 4b. Update the job
		newTime, errParse := time.Parse(time.RFC3339, data.TargetTime)
		if errParse != nil {
			return fmt.Errorf("failed to parse new TargetTime ISO8601: %w", errParse)
		}
		err = a.db.UpdateReminderTime(targetJob.ID, newTime)

		// Calculate relative time for clarity
		duration := time.Until(newTime)
		relativeDesc := ""
		if duration > 0 {
			relativeDesc = fmt.Sprintf(" (in %s)", duration.Round(time.Second))
		}

		responseMsg = fmt.Sprintf("I have updated your reminder '%s' to %s%s", 
			targetJob.ReminderText, newTime.Format("Jan 02, 3:04 PM UTC"), relativeDesc)
	}

	if err != nil {
		return fmt.Errorf("failed to execute database modification: %w", err)
	}

	// 5. Resolve
	ticket.ResolutionNotes = responseMsg
	ticket.UpdateStatus(state.StatusResolved)
	ticket.AddMessage("assistant", responseMsg)
	return nil
}
