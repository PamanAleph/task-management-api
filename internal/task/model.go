package task

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusDone:
		return true
	}
	return false
}

type Task struct {
	ID          int64     `db:"id" json:"id"`
	TeamID      int64     `db:"team_id" json:"team_id"`
	OwnerID     int64     `db:"owner_id" json:"owner_id"`
	AssigneeID  *int64    `db:"assignee_id" json:"assignee_id,omitempty"`
	Title       string    `db:"title" json:"title"`
	Description string    `db:"description" json:"description"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type Log struct {
	ID        int64     `db:"id" json:"id"`
	TaskID    int64     `db:"task_id" json:"task_id"`
	ChangedBy int64     `db:"changed_by" json:"changed_by"`
	Action    string    `db:"action" json:"action"`
	OldValue  []byte    `db:"old_value" json:"old_value,omitempty"`
	NewValue  []byte    `db:"new_value" json:"new_value,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
