package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteTaskStore implements TaskStore backed by a SQLite database.
type SQLiteTaskStore struct {
	db *sql.DB
}

// NewSQLiteTaskStore opens (or creates) a SQLite task database.
func NewSQLiteTaskStore(dbPath string) (*SQLiteTaskStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open tasks database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to tasks database: %w", err)
	}

	s := &SQLiteTaskStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteTaskStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id             TEXT PRIMARY KEY,
		title          TEXT NOT NULL,
		description    TEXT DEFAULT '',
		status         TEXT NOT NULL DEFAULT 'backlog',
		priority       TEXT NOT NULL DEFAULT 'medium',
		assigned_agent TEXT DEFAULT '',
		created_by     TEXT NOT NULL DEFAULT 'system',
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL,
		started_at     TEXT,
		completed_at   TEXT,
		result         TEXT DEFAULT '',
		error          TEXT DEFAULT '',
		labels         TEXT DEFAULT '[]',
		metadata       TEXT DEFAULT '{}',
		parent_id      TEXT DEFAULT '',
		ttl_days       INTEGER,
		template_id    TEXT DEFAULT '',
		approval       INTEGER DEFAULT 0,
		approved_by    TEXT DEFAULT '',
		approved_at    TEXT,
		approval_note  TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_assigned_agent ON tasks(assigned_agent);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
	CREATE INDEX IF NOT EXISTS idx_tasks_parent_id ON tasks(parent_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Create inserts a new task.
func (s *SQLiteTaskStore) Create(t *Task) error {
	if t.ID == "" {
		t.ID = generateID()
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = TaskStatusBacklog
	}
	if t.Priority == "" {
		t.Priority = TaskPriorityMedium
	}
	if t.CreatedBy == "" {
		t.CreatedBy = "system"
	}

	if err := t.Validate(); err != nil {
		return err
	}

	labelsJSON, _ := json.Marshal(t.Labels)
	metadataJSON, _ := json.Marshal(t.Metadata)

	_, err := s.db.Exec(`
		INSERT INTO tasks (
			id, title, description, status, priority, assigned_agent, created_by,
			created_at, updated_at, started_at, completed_at, result, error,
			labels, metadata, parent_id, ttl_days, template_id,
			approval, approved_by, approved_at, approval_note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, string(t.Status), string(t.Priority),
		t.AssignedAgent, t.CreatedBy,
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
		timePtr(t.StartedAt), timePtr(t.CompletedAt),
		t.Result, t.Error,
		string(labelsJSON), string(metadataJSON),
		t.ParentID, t.TTLDays, t.TemplateID,
		boolToInt(t.Approval), t.ApprovedBy, timePtr(t.ApprovedAt), t.ApprovalNote,
	)
	return err
}

// Get retrieves a task by ID.
func (s *SQLiteTaskStore) Get(id string) (*Task, error) {
	row := s.db.QueryRow(`SELECT
		id, title, description, status, priority, assigned_agent, created_by,
		created_at, updated_at, started_at, completed_at, result, error,
		labels, metadata, parent_id, ttl_days, template_id,
		approval, approved_by, approved_at, approval_note
		FROM tasks WHERE id = ?`, id)

	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	return t, err
}

// Update replaces a task's fields.
func (s *SQLiteTaskStore) Update(t *Task) error {
	if err := t.Validate(); err != nil {
		return err
	}

	t.UpdatedAt = time.Now()
	labelsJSON, _ := json.Marshal(t.Labels)
	metadataJSON, _ := json.Marshal(t.Metadata)

	res, err := s.db.Exec(`
		UPDATE tasks SET
			title=?, description=?, status=?, priority=?, assigned_agent=?,
			created_by=?, updated_at=?, started_at=?, completed_at=?,
			result=?, error=?, labels=?, metadata=?, parent_id=?,
			ttl_days=?, template_id=?, approval=?, approved_by=?,
			approved_at=?, approval_note=?
		WHERE id=?`,
		t.Title, t.Description, string(t.Status), string(t.Priority), t.AssignedAgent,
		t.CreatedBy, t.UpdatedAt.Format(time.RFC3339),
		timePtr(t.StartedAt), timePtr(t.CompletedAt),
		t.Result, t.Error, string(labelsJSON), string(metadataJSON), t.ParentID,
		t.TTLDays, t.TemplateID, boolToInt(t.Approval), t.ApprovedBy,
		timePtr(t.ApprovedAt), t.ApprovalNote,
		t.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// Delete removes a task.
func (s *SQLiteTaskStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// List returns tasks matching the filter.
func (s *SQLiteTaskStore) List(filter TaskFilter) ([]*Task, error) {
	var conditions []string
	var args []interface{}

	if filter.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.AssignedAgent != nil {
		conditions = append(conditions, "assigned_agent = ?")
		args = append(args, *filter.AssignedAgent)
	}
	if filter.Priority != nil {
		conditions = append(conditions, "priority = ?")
		args = append(args, string(*filter.Priority))
	}
	if filter.ParentID != nil {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, *filter.ParentID)
	}
	for _, label := range filter.Labels {
		conditions = append(conditions, `labels LIKE ?`)
		args = append(args, fmt.Sprintf(`%%"%s"%%`, label))
	}

	query := `SELECT
		id, title, description, status, priority, assigned_agent, created_by,
		created_at, updated_at, started_at, completed_at, result, error,
		labels, metadata, parent_id, ttl_days, template_id,
		approval, approved_by, approved_at, approval_note
		FROM tasks`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += ` ORDER BY
		CASE priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
		created_at ASC`

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Task
	for rows.Next() {
		t, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// Checkout atomically claims a task for an agent.
func (s *SQLiteTaskStore) Checkout(taskID, agentName string, agentLabels []string) (*Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var query string
	var args []interface{}

	if taskID != "" {
		query = `SELECT
			id, title, description, status, priority, assigned_agent, created_by,
			created_at, updated_at, started_at, completed_at, result, error,
			labels, metadata, parent_id, ttl_days, template_id,
			approval, approved_by, approved_at, approval_note
			FROM tasks WHERE id = ? AND status = 'todo' AND assigned_agent = ''`
		args = []interface{}{taskID}
	} else {
		query = `SELECT
			id, title, description, status, priority, assigned_agent, created_by,
			created_at, updated_at, started_at, completed_at, result, error,
			labels, metadata, parent_id, ttl_days, template_id,
			approval, approved_by, approved_at, approval_note
			FROM tasks WHERE status = 'todo' AND assigned_agent = ''
			ORDER BY
				CASE priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
				created_at ASC
			LIMIT 1`
	}

	row := tx.QueryRow(query, args...)
	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		if taskID != "" {
			// Check if the task exists but is not available
			var exists bool
			tx.QueryRow(`SELECT 1 FROM tasks WHERE id = ?`, taskID).Scan(&exists)
			if exists {
				return nil, ErrTaskAlreadyClaimed
			}
			return nil, ErrTaskNotFound
		}
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	// Check label permissions
	if len(agentLabels) > 0 && len(t.Labels) > 0 {
		if !HasLabelIntersection(agentLabels, t.Labels) {
			return nil, ErrLabelMismatch
		}
	}

	now := time.Now()
	_, err = tx.Exec(`
		UPDATE tasks SET status = 'in_progress', assigned_agent = ?, started_at = ?, updated_at = ?
		WHERE id = ? AND status = 'todo'`,
		agentName, now.Format(time.RFC3339), now.Format(time.RFC3339), t.ID,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	t.Status = TaskStatusInProgress
	t.AssignedAgent = agentName
	t.StartedAt = &now
	t.UpdatedAt = now
	return t, nil
}

// Move changes a task's status with transition validation.
func (s *SQLiteTaskStore) Move(taskID string, newStatus TaskStatus) error {
	if !IsValidStatus(newStatus) {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	t, err := s.Get(taskID)
	if err != nil {
		return err
	}

	if !IsValidTransition(t.Status, newStatus) {
		return ErrInvalidTransition
	}

	if t.Approval && t.Status == TaskStatusReview && newStatus == TaskStatusDone {
		return ErrApprovalRequired
	}

	now := time.Now()
	sets := []string{"status = ?", "updated_at = ?"}
	args := []interface{}{string(newStatus), now.Format(time.RFC3339)}

	if newStatus == TaskStatusDone || newStatus == TaskStatusFailed {
		sets = append(sets, "completed_at = ?")
		args = append(args, now.Format(time.RFC3339))
	}
	if newStatus == TaskStatusInProgress && t.StartedAt == nil {
		sets = append(sets, "started_at = ?")
		args = append(args, now.Format(time.RFC3339))
	}

	args = append(args, taskID)
	_, err = s.db.Exec(
		fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(sets, ", ")),
		args...,
	)
	return err
}

// Approve marks an approval-required task as done.
func (s *SQLiteTaskStore) Approve(taskID, approvedBy, note string) error {
	t, err := s.Get(taskID)
	if err != nil {
		return err
	}

	if t.Status != TaskStatusReview {
		return fmt.Errorf("task must be in review status to approve (current: %s)", t.Status)
	}
	if !t.Approval {
		return fmt.Errorf("task does not require approval")
	}

	now := time.Now()
	_, err = s.db.Exec(`
		UPDATE tasks SET status = 'done', approved_by = ?, approved_at = ?,
		approval_note = ?, completed_at = ?, updated_at = ?
		WHERE id = ?`,
		approvedBy, now.Format(time.RFC3339), note,
		now.Format(time.RFC3339), now.Format(time.RFC3339), taskID,
	)
	return err
}

// Reject marks a task as rejected.
func (s *SQLiteTaskStore) Reject(taskID, rejectedBy, note string) error {
	t, err := s.Get(taskID)
	if err != nil {
		return err
	}

	if t.Status != TaskStatusReview {
		return fmt.Errorf("task must be in review status to reject (current: %s)", t.Status)
	}

	now := time.Now()
	_, err = s.db.Exec(`
		UPDATE tasks SET status = 'rejected', approved_by = ?, approval_note = ?,
		assigned_agent = '', updated_at = ?
		WHERE id = ?`,
		rejectedBy, note, now.Format(time.RFC3339), taskID,
	)
	return err
}

// Stats returns task counts grouped by status.
func (s *SQLiteTaskStore) Stats() (map[TaskStatus]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[TaskStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[TaskStatus(status)] = count
	}
	return stats, rows.Err()
}

// Cleanup removes expired done/failed tasks.
func (s *SQLiteTaskStore) Cleanup(doneTTLDays, failedTTLDays int) (int, error) {
	now := time.Now()
	total := 0

	if doneTTLDays > 0 {
		cutoff := now.Add(-time.Duration(doneTTLDays) * 24 * time.Hour).Format(time.RFC3339)
		res, err := s.db.Exec(`DELETE FROM tasks WHERE status = 'done' AND completed_at IS NOT NULL AND completed_at < ?`, cutoff)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}

	if failedTTLDays > 0 {
		cutoff := now.Add(-time.Duration(failedTTLDays) * 24 * time.Hour).Format(time.RFC3339)
		res, err := s.db.Exec(`DELETE FROM tasks WHERE status = 'failed' AND completed_at IS NOT NULL AND completed_at < ?`, cutoff)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}

	return total, nil
}

// Close closes the database connection.
func (s *SQLiteTaskStore) Close() error {
	return s.db.Close()
}

// --- helpers ---

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanFromScanner(sc scanner) (*Task, error) {
	var t Task
	var status, priority string
	var createdAt, updatedAt string
	var startedAt, completedAt, approvedAt sql.NullString
	var labelsJSON, metadataJSON string
	var ttlDays sql.NullInt64
	var approval int

	err := sc.Scan(
		&t.ID, &t.Title, &t.Description, &status, &priority,
		&t.AssignedAgent, &t.CreatedBy,
		&createdAt, &updatedAt, &startedAt, &completedAt,
		&t.Result, &t.Error,
		&labelsJSON, &metadataJSON,
		&t.ParentID, &ttlDays, &t.TemplateID,
		&approval, &t.ApprovedBy, &approvedAt, &t.ApprovalNote,
	)
	if err != nil {
		return nil, err
	}

	t.Status = TaskStatus(status)
	t.Priority = TaskPriority(priority)
	t.Approval = approval != 0

	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if startedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, startedAt.String)
		t.StartedAt = &ts
	}
	if completedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, completedAt.String)
		t.CompletedAt = &ts
	}
	if approvedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, approvedAt.String)
		t.ApprovedAt = &ts
	}
	if ttlDays.Valid {
		d := int(ttlDays.Int64)
		t.TTLDays = &d
	}

	if labelsJSON != "" && labelsJSON != "[]" {
		json.Unmarshal([]byte(labelsJSON), &t.Labels)
	}
	if metadataJSON != "" && metadataJSON != "{}" {
		json.Unmarshal([]byte(metadataJSON), &t.Metadata)
	}

	return &t, nil
}

func scanTask(row *sql.Row) (*Task, error) {
	return scanFromScanner(row)
}

func scanRows(rows *sql.Rows) (*Task, error) {
	return scanFromScanner(rows)
}

func timePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
