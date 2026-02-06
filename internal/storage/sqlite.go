package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO required)
)

// ExecutionStatus represents the current state of a workflow execution
type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled" // Execution stopped by user
)

// Workflow represents a stored workflow definition
type Workflow struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Definition []byte    `json:"definition"` // Stores the full JSON (engine.Workflow)
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Execution represents a single workflow execution instance

type Execution struct {
	ID          string
	WorkflowID  string
	Status      ExecutionStatus
	State       []byte
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       *string
}

// Trigger represents a workflow trigger configuration
type Trigger struct {
	ID         string    `json:"id"`
	WorkflowID string    `json:"workflow_id"`
	Type       string    `json:"type"`
	Config     []byte    `json:"config"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	FilePath   string    `json:"file_path,omitempty"`
}

// TriggerExecution represents a single trigger firing event
type TriggerExecution struct {
	ID          string
	TriggerID   string
	ExecutionID *string // NULL if workflow execution failed to start
	FiredAt     time.Time
	Status      string // success, failed, skipped
	Payload     []byte // JSON-encoded trigger payload (e.g. webhook body)
	Error       *string
}

// Storage defines the interface for workflow persistence

// This allows tracking full execution history (like n8n)
type Storage interface {

	CreateWorkflow(ctx context.Context, workflow *Workflow) error
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, workflow *Workflow) error
	DeleteWorkflow(ctx context.Context, id string) error
	ListWorkflows(ctx context.Context) ([]*Workflow, error)

	// Execution Management - track history of all workflow runs
	CreateExecution(ctx context.Context, workflowID string) (executionID string, err error)
	UpdateExecutionStatus(ctx context.Context, executionID string, status ExecutionStatus, state []byte, errorMsg *string) error
	GetExecution(ctx context.Context, executionID string) (*Execution, error)
	ListExecutions(ctx context.Context, workflowID string, limit int) ([]*Execution, error)

	// Node Results - now tied to execution_id instead of workflow_id
	SaveNodeResult(ctx context.Context, executionID, nodeID string, result []byte) error
	GetNodeResult(ctx context.Context, executionID, nodeID string) ([]byte, error)

	// Trigger Management
	CreateTrigger(ctx context.Context, trigger *Trigger) error
	GetTrigger(ctx context.Context, id string) (*Trigger, error)
	UpdateTrigger(ctx context.Context, trigger *Trigger) error
	DeleteTrigger(ctx context.Context, id string) error
	ListTriggers(ctx context.Context, workflowID string) ([]*Trigger, error)
	ListAllTriggers(ctx context.Context) ([]*Trigger, error)

	// Trigger Execution History
	CreateTriggerExecution(ctx context.Context, triggerExec *TriggerExecution) error
	ListTriggerExecutions(ctx context.Context, triggerID string, limit int) ([]*TriggerExecution, error)

	Close() error
}

// SQLiteStorage implements Storage using modernc.org/sqlite (Pure Go)
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLite creates a new SQLite-backed storage

func NewSQLite(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

// initSchema creates necessary tables for execution history tracking

func initSchema(db *sql.DB) error {
	schema := `
	-- Workflows: store workflow definitions
	CREATE TABLE IF NOT EXISTS workflows (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		definition BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Execution History: track all workflow runs (not just latest state)
	-- Each workflow run gets a unique execution_id (UUID)
	CREATE TABLE IF NOT EXISTS workflow_executions (
		execution_id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('running', 'completed', 'failed', 'cancelled')),
		state BLOB NOT NULL,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		error TEXT,
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
	);

	-- Index for querying execution history by workflow (most recent first)
	CREATE INDEX IF NOT EXISTS idx_executions_workflow 
		ON workflow_executions(workflow_id, started_at DESC);

	-- Index for querying by status (e.g., find all failed executions)
	CREATE INDEX IF NOT EXISTS idx_executions_status
		ON workflow_executions(status, started_at DESC);

	-- Node Results: now tied to execution_id instead of workflow_id
	-- This allows tracking node outputs for each specific execution
	CREATE TABLE IF NOT EXISTS node_results (
		execution_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		result BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (execution_id, node_id),
		FOREIGN KEY (execution_id) REFERENCES workflow_executions(execution_id) ON DELETE CASCADE
	);

	-- Triggers: store trigger configurations
	CREATE TABLE IF NOT EXISTS triggers (
		id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('cron', 'interval', 'webhook', 'typescript')),
		config BLOB NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		file_path TEXT NOT NULL DEFAULT '', -- New: Stores path to TS file for typescript triggers
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
	);

	-- Index for querying triggers by workflow
	CREATE INDEX IF NOT EXISTS idx_triggers_workflow
		ON triggers(workflow_id);

	-- Index for querying enabled triggers by type
	CREATE INDEX IF NOT EXISTS idx_triggers_type_enabled
		ON triggers(type, enabled);

	-- Trigger Executions: track trigger firing history
	CREATE TABLE IF NOT EXISTS trigger_executions (
		id TEXT PRIMARY KEY,
		trigger_id TEXT NOT NULL,
		execution_id TEXT,
		fired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL CHECK(status IN ('success', 'failed', 'skipped')),
		payload BLOB,
		error TEXT,
		FOREIGN KEY (trigger_id) REFERENCES triggers(id) ON DELETE CASCADE,
		FOREIGN KEY (execution_id) REFERENCES workflow_executions(execution_id) ON DELETE SET NULL
	);

	-- Index for querying trigger execution history
	CREATE INDEX IF NOT EXISTS idx_trigger_executions_trigger
		ON trigger_executions(trigger_id, fired_at DESC);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Idempotent migration to add 'file_path' to triggers table if it doesn't exist.
	rows, err := db.Query("PRAGMA table_info(triggers)")
	if err != nil {
		return fmt.Errorf("failed to get triggers table info: %w", err)
	}
	defer rows.Close()

	var columnExists bool
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notnull    int
			dflt_value *string
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk); err != nil {
			return fmt.Errorf("failed to scan table info row: %w", err)
		}
		if name == "file_path" {
			columnExists = true
			break
		}
	}

	if !columnExists {
		_, err = db.Exec("ALTER TABLE triggers ADD COLUMN file_path TEXT NOT NULL DEFAULT ''")
		if err != nil {
			return fmt.Errorf("failed to add file_path column to triggers table: %w", err)
		}
	}

	return nil
}

// Helper to check if an error is due to a duplicate column name
func isDuplicateColumnError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "duplicate column name") || strings.Contains(err.Error(), "SQLITE_ERROR: duplicate column name"))
}

// --- Workflow CRUD ---

func (s *SQLiteStorage) CreateWorkflow(ctx context.Context, w *Workflow) error {
	query := `
		INSERT INTO workflows (id, name, definition, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := s.db.ExecContext(ctx, query, w.ID, w.Name, w.Definition)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	query := `SELECT id, name, definition, created_at, updated_at FROM workflows WHERE id = ?`
	var w Workflow
	err := s.db.QueryRowContext(ctx, query, id).Scan(&w.ID, &w.Name, &w.Definition, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return &w, nil
}

func (s *SQLiteStorage) UpdateWorkflow(ctx context.Context, w *Workflow) error {
	query := `
		UPDATE workflows 
		SET name = ?, definition = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, w.Name, w.Definition, w.ID)
	if err != nil {
		return fmt.Errorf("failed to update workflow: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("workflow not found")
	}
	return nil
}

func (s *SQLiteStorage) DeleteWorkflow(ctx context.Context, id string) error {
	query := `DELETE FROM workflows WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) ListWorkflows(ctx context.Context) ([]*Workflow, error) {
	query := `SELECT id, name, definition, created_at, updated_at FROM workflows ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []*Workflow
	for rows.Next() {
		var w Workflow
		if err := rows.Scan(&w.ID, &w.Name, &w.Definition, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, &w)
	}
	return workflows, nil
}

// --- Execution Management ---

// CreateExecution creates a new workflow execution instance

func (s *SQLiteStorage) CreateExecution(ctx context.Context, workflowID string) (string, error) {

	// In production, consider using github.com/google/uuid
	executionID := fmt.Sprintf("%s-%d", workflowID, time.Now().UnixNano())

	query := `
		INSERT INTO workflow_executions (execution_id, workflow_id, status, state, started_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := s.db.ExecContext(ctx, query, executionID, workflowID, ExecutionStatusRunning, []byte("{}"))
	if err != nil {
		return "", fmt.Errorf("failed to create execution: %w", err)
	}
	return executionID, nil
}

// UpdateExecutionStatus updates the status and state of an execution

func (s *SQLiteStorage) UpdateExecutionStatus(ctx context.Context, executionID string, status ExecutionStatus, state []byte, errorMsg *string) error {
	query := `
		UPDATE workflow_executions 
		SET status = ?, state = ?, completed_at = CURRENT_TIMESTAMP, error = ?
		WHERE execution_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, status, state, errorMsg, executionID)
	if err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}
	return nil
}

// GetExecution retrieves a specific execution by ID
func (s *SQLiteStorage) GetExecution(ctx context.Context, executionID string) (*Execution, error) {
	query := `
		SELECT execution_id, workflow_id, status, state, started_at, completed_at, error
		FROM workflow_executions
		WHERE execution_id = ?
	`

	var exec Execution
	var completedAt sql.NullTime
	var errorMsg sql.NullString

	err := s.db.QueryRowContext(ctx, query, executionID).Scan(
		&exec.ID,
		&exec.WorkflowID,
		&exec.Status,
		&exec.State,
		&exec.StartedAt,
		&completedAt,
		&errorMsg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	if completedAt.Valid {
		exec.CompletedAt = &completedAt.Time
	}
	if errorMsg.Valid {
		exec.Error = &errorMsg.String
	}

	return &exec, nil
}

// ListExecutions retrieves execution history for a workflow

func (s *SQLiteStorage) ListExecutions(ctx context.Context, workflowID string, limit int) ([]*Execution, error) {
	query := `
		SELECT execution_id, workflow_id, status, state, started_at, completed_at, error
		FROM workflow_executions
		WHERE workflow_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var executions []*Execution
	for rows.Next() {
		var exec Execution
		var completedAt sql.NullTime
		var errorMsg sql.NullString

		err := rows.Scan(
			&exec.ID,
			&exec.WorkflowID,
			&exec.Status,
			&exec.State,
			&exec.StartedAt,
			&completedAt,
			&errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		if completedAt.Valid {
			exec.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			exec.Error = &errorMsg.String
		}

		executions = append(executions, &exec)
	}

	return executions, rows.Err()
}

// SaveNodeResult persists the result of a single node execution

func (s *SQLiteStorage) SaveNodeResult(ctx context.Context, executionID, nodeID string, result []byte) error {
	query := `
		INSERT INTO node_results (execution_id, node_id, result, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(execution_id, node_id) DO UPDATE SET
			result = excluded.result,
			created_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.ExecContext(ctx, query, executionID, nodeID, result)
	if err != nil {
		return fmt.Errorf("failed to save node result: %w", err)
	}
	return nil
}

// GetNodeResult retrieves the result of a specific node execution
func (s *SQLiteStorage) GetNodeResult(ctx context.Context, executionID, nodeID string) ([]byte, error) {
	var result []byte
	query := `SELECT result FROM node_results WHERE execution_id = ? AND node_id = ?`
	err := s.db.QueryRowContext(ctx, query, executionID, nodeID).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to get node result: %w", err)
	}
	return result, nil
}

// --- Trigger Management ---

func (s *SQLiteStorage) CreateTrigger(ctx context.Context, t *Trigger) error {
	query := `
		INSERT INTO triggers (id, workflow_id, type, config, enabled, created_at, updated_at, file_path)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`
	_, err := s.db.ExecContext(ctx, query, t.ID, t.WorkflowID, t.Type, t.Config, t.Enabled, t.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create trigger: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetTrigger(ctx context.Context, id string) (*Trigger, error) {
	query := `SELECT id, workflow_id, type, config, enabled, created_at, updated_at, file_path FROM triggers WHERE id = ?`
	var t Trigger
	err := s.db.QueryRowContext(ctx, query, id).Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Config, &t.Enabled, &t.CreatedAt, &t.UpdatedAt, &t.FilePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trigger not found")
		}
		return nil, fmt.Errorf("failed to get trigger: %w", err)
	}
	return &t, nil
}

func (s *SQLiteStorage) UpdateTrigger(ctx context.Context, t *Trigger) error {
	query := `
		UPDATE triggers 
		SET workflow_id = ?, type = ?, config = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP, file_path = ?
		WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, t.WorkflowID, t.Type, t.Config, t.Enabled, t.FilePath, t.ID)
	if err != nil {
		return fmt.Errorf("failed to update trigger: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("trigger not found")
	}
	return nil
}

func (s *SQLiteStorage) DeleteTrigger(ctx context.Context, id string) error {
	query := `DELETE FROM triggers WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete trigger: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) ListTriggers(ctx context.Context, workflowID string) ([]*Trigger, error) {
	query := `SELECT id, workflow_id, type, config, enabled, created_at, updated_at, file_path FROM triggers WHERE workflow_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}
	defer rows.Close()

	var triggers []*Trigger
	for rows.Next() {
		var t Trigger
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Config, &t.Enabled, &t.CreatedAt, &t.UpdatedAt, &t.FilePath); err != nil {
			return nil, err
		}
		triggers = append(triggers, &t)
	}
	return triggers, nil
}

func (s *SQLiteStorage) ListAllTriggers(ctx context.Context) ([]*Trigger, error) {
	query := `SELECT id, workflow_id, type, config, enabled, created_at, updated_at, file_path FROM triggers WHERE enabled = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list all triggers: %w", err)
	}
	defer rows.Close()

	var triggers []*Trigger
	for rows.Next() {
		var t Trigger
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Config, &t.Enabled, &t.CreatedAt, &t.UpdatedAt, &t.FilePath); err != nil {
			return nil, err
		}
		triggers = append(triggers, &t)
	}
	return triggers, nil
}

// --- Trigger Execution History ---

func (s *SQLiteStorage) CreateTriggerExecution(ctx context.Context, te *TriggerExecution) error {
	query := `
		INSERT INTO trigger_executions (id, trigger_id, execution_id, fired_at, status, payload, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, te.ID, te.TriggerID, te.ExecutionID, te.FiredAt, te.Status, te.Payload, te.Error)
	if err != nil {
		return fmt.Errorf("failed to create trigger execution: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) ListTriggerExecutions(ctx context.Context, triggerID string, limit int) ([]*TriggerExecution, error) {
	query := `
		SELECT id, trigger_id, execution_id, fired_at, status, payload, error
		FROM trigger_executions
		WHERE trigger_id = ?
		ORDER BY fired_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, triggerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list trigger executions: %w", err)
	}
	defer rows.Close()

	var executions []*TriggerExecution
	for rows.Next() {
		var te TriggerExecution
		var executionID sql.NullString
		var payload []byte
		var errorMsg sql.NullString

		err := rows.Scan(
			&te.ID,
			&te.TriggerID,
			&executionID,
			&te.FiredAt,
			&te.Status,
			&payload,
			&errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trigger execution: %w", err)
		}

		if executionID.Valid {
			te.ExecutionID = &executionID.String
		}
		if len(payload) > 0 {
			te.Payload = payload
		}
		if errorMsg.Valid {
			te.Error = &errorMsg.String
		}

		executions = append(executions, &te)
	}
	return executions, nil
}

// Close releases database resources
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
