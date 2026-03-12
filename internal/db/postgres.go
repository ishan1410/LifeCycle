package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

type ReminderJob struct {
	ID           int
	ChatID       int64
	TargetTime   time.Time
	ReminderText string
	Status       string // "pending", "completed", "cancelled"
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbUrl string) (*Database, error) {
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	d := &Database{db: db}
	if err := d.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return d, nil
}

func (d *Database) initTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS reminders (
		id SERIAL PRIMARY KEY,
		chat_id BIGINT NOT NULL,
		target_time TIMESTAMP WITH TIME ZONE NOT NULL,
		reminder_text TEXT NOT NULL,
		status TEXT DEFAULT 'pending'
	);`
	_, err := d.db.Exec(query)
	return err
}

func (d *Database) SaveReminder(chatID int64, targetTime time.Time, text string) (int, error) {
	query := `INSERT INTO reminders (chat_id, target_time, reminder_text, status) VALUES ($1, $2, $3, 'pending') RETURNING id`

	var id int
	err := d.db.QueryRow(query, chatID, targetTime, text).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) GetDueReminders() ([]ReminderJob, error) {
	query := `SELECT id, chat_id, target_time, reminder_text, status FROM reminders WHERE status = 'pending' AND target_time <= $1`
	rows, err := d.db.Query(query, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ReminderJob
	for rows.Next() {
		var j ReminderJob
		if err := rows.Scan(&j.ID, &j.ChatID, &j.TargetTime, &j.ReminderText, &j.Status); err != nil {
			slog.Error("Failed to scan reminder row", "error", err)
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (d *Database) MarkCompleted(id int) error {
	_, err := d.db.Exec(`UPDATE reminders SET status = 'completed' WHERE id = $1`, id)
	return err
}

func (d *Database) MarkCancelled(id int) error {
	_, err := d.db.Exec(`UPDATE reminders SET status = 'cancelled' WHERE id = $1`, id)
	return err
}

// GetPendingRemindersByChatID returns active reminders for a specific user.
func (d *Database) GetPendingRemindersByChatID(chatID int64) ([]ReminderJob, error) {
	query := `SELECT id, chat_id, target_time, reminder_text, status FROM reminders WHERE status = 'pending' AND chat_id = $1 ORDER BY target_time ASC LIMIT 20`

	rows, err := d.db.Query(query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ReminderJob
	for rows.Next() {
		var j ReminderJob
		if err := rows.Scan(&j.ID, &j.ChatID, &j.TargetTime, &j.ReminderText, &j.Status); err != nil {
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (d *Database) UpdateReminderTime(id int, newTime time.Time) error {
	_, err := d.db.Exec(`UPDATE reminders SET target_time = $1 WHERE id = $2`, newTime, id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}
