package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID         string
	TriggerID  string
	WorkflowID string
	Payload    []byte
	Status     string
	Attempts   int
	MaxRetries int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Error      *string
}

func (s *SQLiteStorage) CreateTask(ctx context.Context, task *Task) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	query := `
		INSERT INTO tasks (id, trigger_id, workflow_id, payload, status, attempts, max_retries, created_at, updated_at, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		task.ID, task.TriggerID, task.WorkflowID, task.Payload,
		task.Status, task.Attempts, task.MaxRetries,
		task.CreatedAt, task.UpdatedAt, task.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) UpdateTask(ctx context.Context, task *Task) error {
	query := `
		UPDATE tasks 
		SET status = ?, attempts = ?, updated_at = ?, error = ?
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query,
		task.Status, task.Attempts, task.UpdatedAt, task.Error, task.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetPendingTasks(ctx context.Context, limit int) ([]*Task, error) {
	query := `
		SELECT id, trigger_id, workflow_id, payload, status, attempts, max_retries, created_at, updated_at, error
		FROM tasks
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var task Task
		var errorMsg sql.NullString

		err := rows.Scan(
			&task.ID, &task.TriggerID, &task.WorkflowID, &task.Payload,
			&task.Status, &task.Attempts, &task.MaxRetries,
			&task.CreatedAt, &task.UpdatedAt, &errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if errorMsg.Valid {
			task.Error = &errorMsg.String
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

func (s *SQLiteStorage) CleanupOldExecutions(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM workflow_executions
		WHERE started_at < ? AND status IN ('completed', 'failed', 'cancelled')
	`

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old executions: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		fmt.Printf("Cleaned up %d old executions\n", rows)
	}

	return nil
}
