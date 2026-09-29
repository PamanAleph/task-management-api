package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrAssigneeNotFound = errors.New("assignee not found in the same team")
)

type Repository interface {
	Create(ctx context.Context, t *Task) error
	GetByID(ctx context.Context, id, ownerID int64) (*Task, error)
	List(ctx context.Context, filter ListFilter) (*ListResult, error)
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, id, ownerID int64) error
	// Assign performs the assignee update, the task_logs entry and the
	// notification dispatch inside a single database transaction, rolling
	// back entirely if any step fails.
	Assign(ctx context.Context, taskID, ownerID, assigneeID, changedBy int64, notifier Notifier) (*Task, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, t *Task) error {
	query := `
		INSERT INTO tasks (team_id, owner_id, title, description, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query, t.TeamID, t.OwnerID, t.Title, t.Description, t.Status).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *postgresRepository) GetByID(ctx context.Context, id, ownerID int64) (*Task, error) {
	var t Task
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tasks WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *postgresRepository) List(ctx context.Context, f ListFilter) (*ListResult, error) {
	where := []string{"owner_id = :owner_id"}
	args := map[string]interface{}{
		"owner_id": f.OwnerID,
		"limit":    f.Limit,
		"offset":   (f.Page - 1) * f.Limit,
	}

	if f.Status != "" {
		where = append(where, "status = :status")
		args["status"] = f.Status
	}
	if f.Search != "" {
		where = append(where, "title ILIKE :search")
		args["search"] = "%" + f.Search + "%"
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, whereClause)
	countStmt, err := r.db.PrepareNamedContext(ctx, countQuery)
	if err != nil {
		return nil, err
	}
	defer countStmt.Close()

	var total int64
	if err := countStmt.GetContext(ctx, &total, args); err != nil {
		return nil, err
	}

	listQuery := fmt.Sprintf(`
		SELECT * FROM tasks
		WHERE %s
		ORDER BY created_at DESC
		LIMIT :limit OFFSET :offset`, whereClause)
	listStmt, err := r.db.PrepareNamedContext(ctx, listQuery)
	if err != nil {
		return nil, err
	}
	defer listStmt.Close()

	var items []Task
	rows, err := listStmt.QueryxContext(ctx, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Task
		if err := rows.StructScan(&t); err != nil {
			return nil, err
		}
		items = append(items, t)
	}

	return &ListResult{Items: items, TotalItems: total}, nil
}

func (r *postgresRepository) Update(ctx context.Context, t *Task) error {
	query := `
		UPDATE tasks
		SET title = $1, description = $2, status = $3, updated_at = now()
		WHERE id = $4 AND owner_id = $5
		RETURNING updated_at`
	err := r.db.QueryRowxContext(ctx, query, t.Title, t.Description, t.Status, t.ID, t.OwnerID).Scan(&t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	return err
}

func (r *postgresRepository) Delete(ctx context.Context, id, ownerID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (r *postgresRepository) Assign(ctx context.Context, taskID, ownerID, assigneeID, changedBy int64, notifier Notifier) (*Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var t Task
	err = tx.GetContext(ctx, &t, `SELECT * FROM tasks WHERE id = $1 AND owner_id = $2 FOR UPDATE`, taskID, ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	var assigneeExists bool
	err = tx.GetContext(ctx, &assigneeExists,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND team_id = $2)`, assigneeID, t.TeamID)
	if err != nil {
		return nil, err
	}
	if !assigneeExists {
		return nil, ErrAssigneeNotFound
	}

	oldAssignee := t.AssigneeID

	err = tx.QueryRowxContext(ctx,
		`UPDATE tasks SET assignee_id = $1, updated_at = now() WHERE id = $2 RETURNING assignee_id, updated_at`,
		assigneeID, taskID,
	).Scan(&t.AssigneeID, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}

	oldJSON := fmt.Sprintf(`{"assignee_id": %v}`, nullableInt(oldAssignee))
	newJSON := fmt.Sprintf(`{"assignee_id": %d}`, assigneeID)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO task_logs (task_id, changed_by, action, old_value, new_value) VALUES ($1, $2, $3, $4, $5)`,
		taskID, changedBy, "assign", oldJSON, newJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := notifier.Notify(ctx, taskID, assigneeID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &t, nil
}

func nullableInt(v *int64) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%d", *v)
}
