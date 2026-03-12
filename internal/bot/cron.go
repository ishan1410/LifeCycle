package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/db"
)

// CronProcessor is responsible for polling the SQLite database and executing due jobs.
type CronProcessor struct {
	db          *db.Database
	bot         *TelegramBot
	adminChatID int64
}

func NewCronProcessor(database *db.Database, b *TelegramBot, chatID int64) *CronProcessor {
	return &CronProcessor{
		db:          database,
		bot:         b,
		adminChatID: chatID,
	}
}

// Start spawns the background polling loop
func (c *CronProcessor) Start(ctx context.Context) {
	slog.Info("Starting Cron Processor background loop")
	ticker := time.NewTicker(2 * time.Second) // Check the DB every 2 seconds for faster response

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Cron Processor")
			ticker.Stop()
			return
		case <-ticker.C:
			c.checkDueJobs()
		}
	}
}

func (c *CronProcessor) checkDueJobs() {
	jobs, err := c.db.GetDueReminders()
	if err != nil {
		slog.Error("Cron failed to fetch due reminders", "error", err)
		return
	}

	for _, job := range jobs {
		slog.Info("Executing Scheduled DB Reminder", "job_id", job.ID, "chat_id", job.ChatID, "text", job.ReminderText)

		// 1. Send the Message to the specific user who scheduled it
		c.bot.SendMessage(job.ChatID, "⏰ Reminder: "+job.ReminderText)

		// NOTE: Status was already marked as 'completed' atomically in GetDueReminders
		// to prevent duplication by other bot instances.
	}
}
