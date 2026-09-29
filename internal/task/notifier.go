package task

import (
	"context"

	"github.com/rs/zerolog"
)

// Notifier dispatches an assignment notification. Only a log-based mock is
// implemented here, as required — swap in a real email/push implementation
// later without touching the assign transaction logic.
type Notifier interface {
	Notify(ctx context.Context, taskID, assigneeID int64) error
}

type LogNotifier struct {
	log zerolog.Logger
}

func NewLogNotifier(log zerolog.Logger) *LogNotifier {
	return &LogNotifier{log: log}
}

func (n *LogNotifier) Notify(ctx context.Context, taskID, assigneeID int64) error {
	n.log.Info().
		Int64("task_id", taskID).
		Int64("assignee_id", assigneeID).
		Msg("notification: task assigned (mock)")
	return nil
}
