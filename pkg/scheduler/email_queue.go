// Solo - Personal AI Agent
// License: MIT

package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Swarup012/solo/pkg/db"
	"github.com/Swarup012/solo/pkg/logger"
)

// EmailQueueItem represents an email message in the queue
type EmailQueueItem struct {
	ID        int64
	To        string
	Subject   string
	Body      string
	CreatedAt time.Time
	SentAt    *time.Time
	Error     *string
}

// EmailQueue manages email messages in a SQLite database
type EmailQueue struct {
	db *sql.DB
}

// DefaultDBPath returns the default database path for the email queue
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".octa", "data", "scheduler.db")
}

// NewEmailQueue creates a new EmailQueue instance
func NewEmailQueue(dbPath string) (*EmailQueue, error) {
	database, err := db.Get(dbPath)
	if err != nil {
		return nil, fmt.Errorf("email_queue: failed to get database: %w", err)
	}

	eq := &EmailQueue{db: database}

	// Initialize the schema
	if err := eq.initSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("email_queue: failed to initialize schema: %w", err)
	}

	return eq, nil
}

// initSchema creates the email_queue table if it doesn't exist
func (eq *EmailQueue) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS email_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		to_address TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME,
		error TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_email_queue_sent_at ON email_queue(sent_at);
	`

	_, err := eq.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Enqueue adds an email to the queue
func (eq *EmailQueue) Enqueue(ctx context.Context, to, subject, body string) error {
	query := `INSERT INTO email_queue (to_address, subject, body) VALUES (?, ?, ?)`
	_, err := eq.db.ExecContext(ctx, query, to, subject, body)
	if err != nil {
		return fmt.Errorf("failed to enqueue email: %w", err)
	}

	logger.InfoCF("email_queue", "Email enqueued", map[string]any{
		"to":      to,
		"subject": subject,
	})

	return nil
}

// GetPending returns all unsent emails in the queue
func (eq *EmailQueue) GetPending(ctx context.Context) ([]EmailQueueItem, error) {
	query := `SELECT id, to_address, subject, body, created_at FROM email_queue WHERE sent_at IS NULL ORDER BY created_at ASC`

	rows, err := eq.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending emails: %w", err)
	}
	defer rows.Close()

	var items []EmailQueueItem
	for rows.Next() {
		var item EmailQueueItem
		if err := rows.Scan(&item.ID, &item.To, &item.Subject, &item.Body, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating emails: %w", err)
	}

	return items, nil
}

// MarkSent marks an email as successfully sent
func (eq *EmailQueue) MarkSent(ctx context.Context, id int64) error {
	query := `UPDATE email_queue SET sent_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := eq.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark email as sent: %w", err)
	}
	return nil
}

// MarkError marks an email as failed with an error message
func (eq *EmailQueue) MarkError(ctx context.Context, id int64, errMsg string) error {
	query := `UPDATE email_queue SET error = ? WHERE id = ?`
	_, err := eq.db.ExecContext(ctx, query, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to mark email error: %w", err)
	}
	return nil
}

// Enqueue adds an email to the queue.

// GetPendingCount returns the count of pending emails.
func (q *EmailQueue) GetPendingCount(ctx context.Context) (int, error) {
	var count int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_queue WHERE sent_at IS NULL`).Scan(&count)
	return count, err
}
