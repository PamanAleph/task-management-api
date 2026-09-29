package task

type CreateRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
}

type UpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type AssignRequest struct {
	AssigneeID int64 `json:"assignee_id" validate:"required"`
}

type ListFilter struct {
	OwnerID int64
	Status  string
	Search  string
	Page    int
	Limit   int
}

type ListResult struct {
	Items      []Task
	TotalItems int64
	// Page/Limit are the values actually applied to the query, after
	// clamping — callers must build response metadata from these, not
	// from the raw request filter, or pagination info drifts from what
	// was actually queried.
	Page  int
	Limit int
}
