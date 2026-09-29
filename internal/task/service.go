package task

import (
	"context"
	"errors"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
)

type Service struct {
	repo     Repository
	notifier Notifier
}

func NewService(repo Repository, notifier Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

func (s *Service) Create(ctx context.Context, teamID, ownerID int64, req CreateRequest) (*Task, *apperror.AppError) {
	if req.Title == "" {
		return nil, apperror.ErrValidation("title is required")
	}

	t := &Task{
		TeamID:      teamID,
		OwnerID:     ownerID,
		Title:       req.Title,
		Description: req.Description,
		Status:      string(StatusPending),
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, apperror.ErrInternal(err)
	}
	return t, nil
}

func (s *Service) Get(ctx context.Context, id, ownerID int64) (*Task, *apperror.AppError) {
	t, err := s.repo.GetByID(ctx, id, ownerID)
	if errors.Is(err, ErrTaskNotFound) {
		return nil, apperror.ErrNotFound("task not found")
	}
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}
	return t, nil
}

func (s *Service) List(ctx context.Context, f ListFilter) (*ListResult, *apperror.AppError) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Status != "" && !Status(f.Status).Valid() {
		return nil, apperror.ErrValidation("invalid status filter")
	}

	res, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}
	res.Page = f.Page
	res.Limit = f.Limit
	return res, nil
}

func (s *Service) Update(ctx context.Context, id, ownerID int64, req UpdateRequest) (*Task, *apperror.AppError) {
	t, err := s.repo.GetByID(ctx, id, ownerID)
	if errors.Is(err, ErrTaskNotFound) {
		return nil, apperror.ErrNotFound("task not found")
	}
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}

	if req.Title != nil {
		if *req.Title == "" {
			return nil, apperror.ErrValidation("title cannot be empty")
		}
		t.Title = *req.Title
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Status != nil {
		if !Status(*req.Status).Valid() {
			return nil, apperror.ErrValidation("invalid status")
		}
		t.Status = *req.Status
	}

	if err := s.repo.Update(ctx, t); err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return nil, apperror.ErrNotFound("task not found")
		}
		return nil, apperror.ErrInternal(err)
	}
	return t, nil
}

func (s *Service) Delete(ctx context.Context, id, ownerID int64) *apperror.AppError {
	if err := s.repo.Delete(ctx, id, ownerID); err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return apperror.ErrNotFound("task not found")
		}
		return apperror.ErrInternal(err)
	}
	return nil
}

func (s *Service) Assign(ctx context.Context, taskID, ownerID, assigneeID, changedBy int64) (*Task, *apperror.AppError) {
	t, err := s.repo.Assign(ctx, taskID, ownerID, assigneeID, changedBy, s.notifier)
	if errors.Is(err, ErrTaskNotFound) {
		return nil, apperror.ErrNotFound("task not found")
	}
	if errors.Is(err, ErrAssigneeNotFound) {
		return nil, apperror.ErrValidation("assignee must belong to the same team")
	}
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}
	return t, nil
}
