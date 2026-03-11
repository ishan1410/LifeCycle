package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type ReminderJob struct {
	ID           int
	TargetTime   time.Time
	ReminderText string
	Status       string // "pending", "completed", "cancelled"
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
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
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_time DATETIME NOT NULL,
		reminder_text TEXT NOT NULL,
		status TEXT DEFAULT 'pending'
	);`
	_, err := d.db.Exec(query)
	return err
}

func (d *Database) SaveReminder(targetTime time.Time, text string) (int, error) {
	query := `INSERT INTO reminders (target_time, reminder_text, status) VALUES (?, ?, 'pending')`
	res, err := d.db.Exec(query, targetTime, text)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (d *Database) GetDueReminders() ([]ReminderJob, error) {
	query := `SELECT id, target_time, reminder_text, status FROM reminders WHERE status = 'pending' AND target_time <= ?`
	rows, err := d.db.Query(query, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ReminderJob
	for rows.Next() {
		var j ReminderJob
		var targetTime string
		if err := rows.Scan(&j.ID, &targetTime, &j.ReminderText, &j.Status); err != nil {
			slog.Error("Failed to scan reminder row", "error", err)
			continue
		}

		t, err := time.Parse(time.RFC3339, targetTime)
		if err == nil {
			j.TargetTime = t
		}

		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (d *Database) MarkCompleted(id int) error {
	_, err := d.db.Exec(`UPDATE reminders SET status = 'completed' WHERE id = ?`, id)
	return err
}

func (d *Database) MarkCancelled(id int) error {
	_, err := d.db.Exec(`UPDATE reminders SET status = 'cancelled' WHERE id = ?`, id)
	return err
}

// GetAllPendingReminders returns all active reminders so the LLM can choose the correct one from a list.
func (d *Database) GetAllPendingReminders() ([]ReminderJob, error) {
	query := `SELECT id, target_time, reminder_text, status FROM reminders WHERE status = 'pending' ORDER BY target_time ASC LIMIT 20`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ReminderJob
	for rows.Next() {
		var j ReminderJob
		var targetTime string
		if err := rows.Scan(&j.ID, &targetTime, &j.ReminderText, &j.Status); err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, targetTime)
		j.TargetTime = t
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (d *Database) UpdateReminderTime(id int, newTime time.Time) error {
	_, err := d.db.Exec(`UPDATE reminders SET target_time = ? WHERE id = ?`, newTime, id)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}
