
package service

import (
	"context"
	"errors"
	"math"
	"task-management-system/dto"
	"task-management-system/models"
	"task-management-system/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Sentinel errors ─────────────────────────────────────────────────────────

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrUnauthorized     = errors.New("you do not have permission to perform this action")
	ErrInvalidID        = errors.New("invalid id format")
	ErrAssigneeNotFound = errors.New("no user found with that assignee email")
)

// ─── Interface ───────────────────────────────────────────────────────────────

type TaskService interface {
	CreateTask(ctx context.Context, requesterID primitive.ObjectID, req dto.CreateTaskRequest) (*dto.TaskDetail, error)
	GetTaskByID(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) (*dto.TaskDetail, error)
	ListTasks(ctx context.Context, requesterID primitive.ObjectID, params dto.TaskFilterParams) (*dto.PaginatedTasksResponse, error)
	UpdateTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, req dto.UpdateTaskRequest) (*dto.TaskDetail, error)
	DeleteTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) error
}

// ─── Implementation ──────────────────────────────────────────────────────────

type taskService struct {
	taskRepo repository.TaskRepository
	userRepo repository.UserRepository
}

func NewTaskService(taskRepo repository.TaskRepository, userRepo repository.UserRepository) TaskService {
	return &taskService{taskRepo: taskRepo, userRepo: userRepo}
}

// ─── Private helpers ─────────────────────────────────────────────────────────

func toUserSummary(u *models.User) *dto.UserSummary {
	if u == nil {
		return nil
	}
	return &dto.UserSummary{ID: u.ID, Name: u.Name, Email: u.Email}
}

// resolveAssigneeEmail looks up a user by email and returns their ObjectID.
// Returns ErrAssigneeNotFound (mapped to 404 by the controller) when the
// email does not match any registered user.
func (s *taskService) resolveAssigneeEmail(ctx context.Context, email string) (primitive.ObjectID, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return primitive.NilObjectID, ErrAssigneeNotFound
		}
		return primitive.NilObjectID, err
	}
	return user.ID, nil
}

// hydrateUsers fetches all referenced user documents in one DB round-trip
// and returns them as a map[ObjectID → *User] for O(1) access when building
// TaskSummary / TaskDetail responses.
func (s *taskService) hydrateUsers(ctx context.Context, tasks []models.Task) (map[primitive.ObjectID]*models.User, error) {
	idSet := make(map[primitive.ObjectID]struct{})
	for _, t := range tasks {
		idSet[t.Assignee] = struct{}{}
		idSet[t.AssignedBy] = struct{}{}
	}

	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		if !id.IsZero() {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return map[primitive.ObjectID]*models.User{}, nil
	}
	return s.userRepo.FindByIDs(ctx, ids)
}

func toTaskSummary(t *models.Task, users map[primitive.ObjectID]*models.User) dto.TaskSummary {
	return dto.TaskSummary{
		ID:         t.ID,
		Title:      t.Title,
		Summary:    t.Summary,
		Type:       string(t.Type),
		Labels:     t.Labels,
		Status:     string(t.Status),
		Priority:   string(t.Priority),
		Assignee:   toUserSummary(users[t.Assignee]),
		AssignedBy: toUserSummary(users[t.AssignedBy]),
		StartDate:  t.StartDate,
		DueDate:    t.DueDate,
		CreatedAt:  t.CreatedAt,
	}
}

func toTaskDetail(t *models.Task, users map[primitive.ObjectID]*models.User) *dto.TaskDetail {
	return &dto.TaskDetail{
		ID:          t.ID,
		Title:       t.Title,
		Summary:     t.Summary,
		Description: t.Description,
		Type:        string(t.Type),
		Labels:      t.Labels,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		Assignee:    toUserSummary(users[t.Assignee]),
		AssignedBy:  toUserSummary(users[t.AssignedBy]),
		StartDate:   t.StartDate,
		DueDate:     t.DueDate,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// canAccess returns true if the user is either the assignee or the creator.
func canAccess(task *models.Task, userID primitive.ObjectID) bool {
	return task.Assignee == userID || task.AssignedBy == userID
}

// ─── Service methods ─────────────────────────────────────────────────────────

// CreateTask inserts a new task.
// assignee_email in the request is resolved to an ObjectID here before writing.
// The caller automatically becomes assigned_by.
func (s *taskService) CreateTask(ctx context.Context, requesterID primitive.ObjectID, req dto.CreateTaskRequest) (*dto.TaskDetail, error) {
	// Resolve assignee email → ObjectID
	assigneeOID, err := s.resolveAssigneeEmail(ctx, req.AssigneeEmail)
	if err != nil {
		return nil, err // ErrAssigneeNotFound or DB error
	}

	status := models.StatusToDo
	if req.Status != "" {
		status = models.TaskStatus(req.Status)
	}

	labels := req.Labels
	if labels == nil {
		labels = []string{}
	}

	task := &models.Task{
		Title:       req.Title,
		Summary:     req.Summary,
		Description: req.Description,
		Type:        models.TaskType(req.Type),
		Labels:      labels,
		Status:      status,
		Priority:    models.TaskPriority(req.Priority),
		Assignee:    assigneeOID,
		AssignedBy:  requesterID,
		StartDate:   req.StartDate,
		DueDate:     req.DueDate,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	users, err := s.hydrateUsers(ctx, []models.Task{*task})
	if err != nil {
		return nil, err
	}

	return toTaskDetail(task, users), nil
}

// GetTaskByID returns full task details.
// Only the assignee or assigned_by may view; others receive ErrUnauthorized.
func (s *taskService) GetTaskByID(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) (*dto.TaskDetail, error) {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	if !canAccess(task, requesterID) {
		return nil, ErrUnauthorized
	}

	users, err := s.hydrateUsers(ctx, []models.Task{*task})
	if err != nil {
		return nil, err
	}

	return toTaskDetail(task, users), nil
}

// ListTasks returns paginated TaskSummary items visible to the requester
// (tasks where they are either assignee or assigned_by).
func (s *taskService) ListTasks(ctx context.Context, requesterID primitive.ObjectID, params dto.TaskFilterParams) (*dto.PaginatedTasksResponse, error) {
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 10
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	tasks, total, err := s.taskRepo.FindAll(ctx, requesterID, params)
	if err != nil {
		return nil, err
	}

	users, err := s.hydrateUsers(ctx, tasks)
	if err != nil {
		return nil, err
	}

	summaries := make([]dto.TaskSummary, 0, len(tasks))
	for i := range tasks {
		summaries = append(summaries, toTaskSummary(&tasks[i], users))
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.PaginatedTasksResponse{
		Tasks:      summaries,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateTask applies a partial update. Only assigned_by (the creator) may edit.
// If assignee_email is provided, it is resolved to an ObjectID before patching.
func (s *taskService) UpdateTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, req dto.UpdateTaskRequest) (*dto.TaskDetail, error) {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	// Only the creator may update
	if task.AssignedBy != requesterID {
		return nil, ErrUnauthorized
	}

	// Build the $set patch — only fields that were explicitly provided
	patch := bson.M{}

	if req.Title != nil {
		patch["title"] = *req.Title
	}
	if req.Summary != nil {
		patch["summary"] = *req.Summary
	}
	if req.Description != nil {
		patch["description"] = *req.Description
	}
	if req.Type != nil {
		patch["type"] = *req.Type
	}
	if req.Labels != nil {
		patch["labels"] = req.Labels
	}
	if req.Status != nil {
		patch["status"] = *req.Status
	}
	if req.Priority != nil {
		patch["priority"] = *req.Priority
	}

	// Resolve the new assignee by email if provided
	if req.AssigneeEmail != nil {
		newAssigneeOID, err := s.resolveAssigneeEmail(ctx, *req.AssigneeEmail)
		if err != nil {
			return nil, err // ErrAssigneeNotFound or DB error
		}
		patch["assignee"] = newAssigneeOID
	}

	if req.StartDate != nil {
		patch["start_date"] = *req.StartDate
	}
	if req.DueDate != nil {
		patch["due_date"] = *req.DueDate
	}

	// Nothing changed — return current state without a DB write
	if len(patch) == 0 {
		users, _ := s.hydrateUsers(ctx, []models.Task{*task})
		return toTaskDetail(task, users), nil
	}

	if err := s.taskRepo.Update(ctx, taskID, patch); err != nil {
		return nil, err
	}

	// Re-fetch to return the fully up-to-date document
	updated, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	users, err := s.hydrateUsers(ctx, []models.Task{*updated})
	if err != nil {
		return nil, err
	}

	return toTaskDetail(updated, users), nil
}

// DeleteTask soft-deletes a task. Only assigned_by may delete.
func (s *taskService) DeleteTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTaskNotFound
		}
		return err
	}

	if task.AssignedBy != requesterID {
		return ErrUnauthorized
	}

	return s.taskRepo.SoftDelete(ctx, taskID)
}

// Compile-time interface check
var _ TaskService = (*taskService)(nil)
