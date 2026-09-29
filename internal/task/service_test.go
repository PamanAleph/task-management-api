package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	tasks  map[int64]*Task
	nextID int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{tasks: make(map[int64]*Task)}
}

func (m *mockRepo) Create(_ context.Context, t *Task) error {
	m.nextID++
	t.ID = m.nextID
	cp := *t
	m.tasks[t.ID] = &cp
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id, ownerID int64) (*Task, error) {
	t, ok := m.tasks[id]
	if !ok || t.OwnerID != ownerID {
		return nil, ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

func (m *mockRepo) List(_ context.Context, f ListFilter) (*ListResult, error) {
	var items []Task
	for _, t := range m.tasks {
		if t.OwnerID != f.OwnerID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		items = append(items, *t)
	}
	return &ListResult{Items: items, TotalItems: int64(len(items))}, nil
}

func (m *mockRepo) Update(_ context.Context, t *Task) error {
	existing, ok := m.tasks[t.ID]
	if !ok || existing.OwnerID != t.OwnerID {
		return ErrTaskNotFound
	}
	cp := *t
	m.tasks[t.ID] = &cp
	return nil
}

func (m *mockRepo) Delete(_ context.Context, id, ownerID int64) error {
	existing, ok := m.tasks[id]
	if !ok || existing.OwnerID != ownerID {
		return ErrTaskNotFound
	}
	delete(m.tasks, id)
	return nil
}

func (m *mockRepo) Assign(_ context.Context, taskID, ownerID, assigneeID, changedBy int64, notifier Notifier) (*Task, error) {
	existing, ok := m.tasks[taskID]
	if !ok || existing.OwnerID != ownerID {
		return nil, ErrTaskNotFound
	}
	existing.AssigneeID = &assigneeID
	if err := notifier.Notify(context.Background(), taskID, assigneeID); err != nil {
		return nil, err
	}
	cp := *existing
	return &cp, nil
}

type mockNotifier struct {
	called int
}

func (n *mockNotifier) Notify(_ context.Context, _, _ int64) error {
	n.called++
	return nil
}

func TestService_Create_RequiresTitle(t *testing.T) {
	svc := NewService(newMockRepo(), &mockNotifier{})
	_, err := svc.Create(context.Background(), 1, 1, CreateRequest{Title: ""})
	require.NotNil(t, err)
	assert.Equal(t, "VALIDATION_ERROR", err.Code)
}

func TestService_Create_DefaultsToPendingStatus(t *testing.T) {
	svc := NewService(newMockRepo(), &mockNotifier{})
	tsk, err := svc.Create(context.Background(), 1, 42, CreateRequest{Title: "write tests"})
	require.Nil(t, err)
	assert.Equal(t, string(StatusPending), tsk.Status)
	assert.Equal(t, int64(42), tsk.OwnerID)
}

func TestService_Get_NotFoundForOtherOwner(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockNotifier{})
	created, _ := svc.Create(context.Background(), 1, 1, CreateRequest{Title: "a"})

	_, err := svc.Get(context.Background(), created.ID, 999)
	require.NotNil(t, err)
	assert.Equal(t, "NOT_FOUND", err.Code)
}

func TestService_Update_InvalidStatusRejected(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockNotifier{})
	created, _ := svc.Create(context.Background(), 1, 1, CreateRequest{Title: "a"})

	bogus := "not_a_status"
	_, err := svc.Update(context.Background(), created.ID, 1, UpdateRequest{Status: &bogus})
	require.NotNil(t, err)
	assert.Equal(t, "VALIDATION_ERROR", err.Code)
}

func TestService_Assign_CallsNotifierExactlyOnce(t *testing.T) {
	repo := newMockRepo()
	notifier := &mockNotifier{}
	svc := NewService(repo, notifier)
	created, _ := svc.Create(context.Background(), 1, 1, CreateRequest{Title: "a"})

	tsk, err := svc.Assign(context.Background(), created.ID, 1, 2, 1)
	require.Nil(t, err)
	require.NotNil(t, tsk.AssigneeID)
	assert.Equal(t, int64(2), *tsk.AssigneeID)
	assert.Equal(t, 1, notifier.called)
}

func TestService_Delete_NotFoundForWrongOwner(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockNotifier{})
	created, _ := svc.Create(context.Background(), 1, 1, CreateRequest{Title: "a"})

	err := svc.Delete(context.Background(), created.ID, 999)
	require.NotNil(t, err)
	assert.Equal(t, "NOT_FOUND", err.Code)
}
