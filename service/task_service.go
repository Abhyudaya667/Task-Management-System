package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"task-management-system/dto"
	"task-management-system/models"
	"task-management-system/repository"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Sentinel errors ─────────────────────────────────────────────────────────

var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrUnauthorized      = errors.New("you do not have permission to perform this action")
	ErrInvalidID         = errors.New("invalid id format")
	ErrAssigneeNotFound  = errors.New("no user found with the UserName")
	ErrDateInPast        = errors.New("start_date and due_date must be in the future")
	ErrInvalidDateRange  = errors.New("due_date must be after start_date")
	ErrCommentNotFound   = errors.New("comment not found")
	ErrCommentForbidden  = errors.New("you are not the author of this comment")
)

// ─── Interface ───────────────────────────────────────────────────────────────

type TaskService interface {
	CreateTask(ctx context.Context, requesterID primitive.ObjectID, req dto.CreateTaskRequest) (*dto.TaskDetail, error)
	GetTaskByID(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) (*dto.TaskDetail, error)
	ListTasks(ctx context.Context, requesterID primitive.ObjectID, params dto.TaskFilterParams) (*dto.PaginatedTasksResponse, error)
	UpdateTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, req dto.UpdateTaskRequest) (*dto.TaskDetail, error)
	DeleteTask(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) error
	GetTaskActivities(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) ([]dto.ActivityResponse, error)

	// Comment operations
	AddComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, req dto.CreateCommentRequest) (*dto.CommentResponse, error)
	EditComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, commentID primitive.ObjectID, req dto.UpdateCommentRequest) (*dto.CommentResponse, error)
	DeleteComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, commentID primitive.ObjectID) error
	GetTaskComments(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) ([]dto.TimelineItem, error)
}

// ─── Implementation ──────────────────────────────────────────────────────────

type taskService struct {
	taskRepo     repository.TaskRepository
	userRepo     repository.UserRepository
	activityRepo repository.ActivityRepository
	commentRepo  repository.CommentInterface
}

func NewTaskService(
	taskRepo repository.TaskRepository,
	userRepo repository.UserRepository,
	activityRepo repository.ActivityRepository,
	commentRepo repository.CommentInterface,
) TaskService {
	return &taskService{
		taskRepo:     taskRepo,
		userRepo:     userRepo,
		activityRepo: activityRepo,
		commentRepo:  commentRepo,
	}
}

// ─── Private helpers ─────────────────────────────────────────────────────────

// resolveAssigneeUserName looks up a user by username and returns their ObjectID.
func (s *taskService) resolveAssigneeUserName(ctx context.Context, username string) (primitive.ObjectID, error) {
	user, err := s.userRepo.FindByUserName(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return primitive.NilObjectID, ErrAssigneeNotFound
		}
		return primitive.NilObjectID, err
	}
	return user.ID, nil
}

// hydrateUsers fetches all referenced user documents in one DB round-trip
// and returns them as a map[ObjectID → *User] for O(1) access.
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

// hydrateUserIDs batch-fetches users by a plain list of ObjectIDs.
func (s *taskService) hydrateUserIDs(ctx context.Context, ids []primitive.ObjectID) map[primitive.ObjectID]*models.User {
	deduped := make([]primitive.ObjectID, 0, len(ids))
	seen := make(map[primitive.ObjectID]struct{})
	for _, id := range ids {
		if !id.IsZero() {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				deduped = append(deduped, id)
			}
		}
	}
	if len(deduped) == 0 {
		return map[primitive.ObjectID]*models.User{}
	}
	usersMap, err := s.userRepo.FindByIDs(ctx, deduped)
	if err != nil {
		return map[primitive.ObjectID]*models.User{}
	}
	return usersMap
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

// logActivity is a fire-and-forget helper for inserting an activity record.
func (s *taskService) logActivity(ctx context.Context, taskID primitive.ObjectID, userID primitive.ObjectID, message string) {
	_ = s.activityRepo.Create(ctx, &models.Activity{
		TaskID:  taskID,
		UserID:  userID,
		Message: message,
	})
}

// ─── Task service methods ─────────────────────────────────────────────────────

// CreateTask inserts a new task and logs a "Task created" activity.
func (s *taskService) CreateTask(ctx context.Context, requesterID primitive.ObjectID, req dto.CreateTaskRequest) (*dto.TaskDetail, error) {
	// ── Date validation ───────────────────────────────────────────────────────
	// Both dates must be strictly in the future and due_date must be after start_date.
	// now := time.Now()
	// if !req.StartDate.After(now) || !req.DueDate.After(now) {
	// 	return nil, ErrDateInPast
	// }
	if !req.DueDate.After(req.StartDate) && !req.DueDate.Equal(req.StartDate) {
		return nil, ErrInvalidDateRange
	}

	assigneeOID, err := s.resolveAssigneeUserName(ctx, req.AssigneeUserName)
	if err != nil {
		return nil, err
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
		Title:         req.Title,
		Summary:       req.Summary,
		Description:   req.Description,
		Type:          models.TaskType(req.Type),
		Labels:        labels,
		Status:        status,
		Priority:      models.TaskPriority(req.Priority),
		PrioritySetAt: time.Now(), // anchor for the auto-escalation timer
		Assignee:      assigneeOID,
		AssignedBy:    requesterID,
		StartDate:     req.StartDate,
		DueDate:       req.DueDate,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	// Log creation activity
	s.logActivity(ctx, task.ID, requesterID, "Task created")

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

// ListTasks returns paginated TaskSummary items visible to the requester.
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
// Generates detailed activity log entries for each changed field.
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
	var changes []string

	if req.Title != nil {
		patch["title"] = *req.Title
		if *req.Title != task.Title {
			changes = append(changes, fmt.Sprintf("Title changed from '%s' to '%s'", task.Title, *req.Title))
		}
	}
	if req.Summary != nil {
		patch["summary"] = *req.Summary
		if *req.Summary != task.Summary {
			changes = append(changes, "Summary updated")
		}
	}
	if req.Description != nil {
		patch["description"] = *req.Description
		if *req.Description != task.Description {
			changes = append(changes, "Description updated")
		}
	}
	if req.Type != nil {
		patch["type"] = *req.Type
		if models.TaskType(*req.Type) != task.Type {
			changes = append(changes, fmt.Sprintf("Type changed from '%s' to '%s'", task.Type, *req.Type))
		}
	}
	if req.Labels != nil {
		patch["labels"] = req.Labels
		changes = append(changes, "Labels updated")
	}
	if req.Status != nil {
		patch["status"] = *req.Status
		if models.TaskStatus(*req.Status) != task.Status {
			changes = append(changes, fmt.Sprintf("Status changed from '%s' to '%s'", task.Status, *req.Status))
		}
	}
	if req.Priority != nil {
		patch["priority"] = *req.Priority
		if models.TaskPriority(*req.Priority) != task.Priority {
			changes = append(changes, fmt.Sprintf("Priority changed from '%s' to '%s'", task.Priority, *req.Priority))
			// Reset the escalation timer whenever the user manually changes priority.
			patch["priority_set_at"] = time.Now()
		}
	}

	// Resolve the new assignee by username if provided
	if req.AssigneeUserName != "" {
		newAssigneeOID, err := s.resolveAssigneeUserName(ctx, req.AssigneeUserName)
		if err != nil {
			return nil, err
		}
		patch["assignee"] = newAssigneeOID
		if newAssigneeOID != task.Assignee {
			changes = append(changes, fmt.Sprintf("Assignee changed to '%s'", req.AssigneeUserName))
		}
	}

	if req.StartDate != nil {
		patch["start_date"] = *req.StartDate
		if !req.StartDate.Equal(task.StartDate) {
			changes = append(changes, fmt.Sprintf("Start date changed to '%s'", req.StartDate.Format("2006-01-02")))
		}
	}
	if req.DueDate != nil {
		patch["due_date"] = *req.DueDate
		if !req.DueDate.Equal(task.DueDate) {
			changes = append(changes, fmt.Sprintf("Due date changed to '%s'", req.DueDate.Format("2006-01-02")))
			// Reset sent notifications so reminders re-fire based on the new due date.
			patch["notifications_sent"] = []string{}
		}
	}

	// ── Date ordering validation (update only checks ordering, not future) ────
	// Resolve the effective start_date and due_date after applying the patch.
	effectiveStart := task.StartDate
	if req.StartDate != nil {
		effectiveStart = *req.StartDate
	}
	effectiveDue := task.DueDate
	if req.DueDate != nil {
		effectiveDue = *req.DueDate
	}
	if req.StartDate != nil || req.DueDate != nil {
		if !effectiveDue.After(effectiveStart) && !effectiveDue.Equal(effectiveStart) {
			return nil, ErrInvalidDateRange
		}
	}

	// Nothing changed — return current state without a DB write
	if len(patch) == 0 {
		users, _ := s.hydrateUsers(ctx, []models.Task{*task})
		return toTaskDetail(task, users), nil
	}

	if err := s.taskRepo.Update(ctx, taskID, patch); err != nil {
		return nil, err
	}

	// Log activity for each changed field
	if len(changes) > 0 {
		s.logActivity(ctx, taskID, requesterID, "Task updated: "+strings.Join(changes, ", "))
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

	if err := s.taskRepo.SoftDelete(ctx, taskID); err != nil {
		return err
	}

	s.logActivity(ctx, taskID, requesterID, "Task deleted")
	return nil
}

// GetTaskActivities returns the full activity history for a task.
// Only the assignee or assigned_by may view.
func (s *taskService) GetTaskActivities(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) ([]dto.ActivityResponse, error) {
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

	activities, err := s.activityRepo.FindByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Collect unique user IDs from activities to hydrate in one round-trip
	idSet := make(map[primitive.ObjectID]struct{})
	for _, act := range activities {
		if !act.UserID.IsZero() {
			idSet[act.UserID] = struct{}{}
		}
	}
	userIDs := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		userIDs = append(userIDs, id)
	}

	usersMap := make(map[primitive.ObjectID]*models.User)
	if len(userIDs) > 0 {
		if fetched, err := s.userRepo.FindByIDs(ctx, userIDs); err == nil {
			usersMap = fetched
		}
	}

	response := make([]dto.ActivityResponse, 0, len(activities))
	for _, act := range activities {
		resp := dto.ActivityResponse{
			ID:        act.ID,
			TaskID:    act.TaskID,
			Message:   act.Message,
			CreatedAt: act.CreatedAt,
		}
		if u, ok := usersMap[act.UserID]; ok {
			resp.User = toUserSummary(u)
		}
		response = append(response, resp)
	}

	return response, nil
}

// ─── Comment service methods ──────────────────────────────────────────────────

// AddComment creates a new user comment on a task.
// Both the assignee and assigned_by may comment.
func (s *taskService) AddComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, req dto.CreateCommentRequest) (*dto.CommentResponse, error) {
	// Verify task exists and requester has access
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

	comment := &models.Comment{
		TaskID:  taskID,
		UserID:  requesterID,
		Message: req.Message,
	}
	if err := s.commentRepo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	// Hydrate author for response
	usersMap := s.hydrateUserIDs(ctx, []primitive.ObjectID{requesterID})

	return toCommentResponse(comment, usersMap), nil
}

// EditComment updates the message of a comment. Only the original author may edit.
func (s *taskService) EditComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, commentID primitive.ObjectID, req dto.UpdateCommentRequest) (*dto.CommentResponse, error) {
	// Verify task exists and requester has access
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

	// Fetch the comment to check ownership
	comment, err := s.commentRepo.FindByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}

	// Only the original author may edit
	if comment.UserID != requesterID {
		return nil, ErrCommentForbidden
	}

	// Ensure this comment belongs to the task in the URL
	if comment.TaskID != taskID {
		return nil, ErrCommentNotFound
	}

	if err := s.commentRepo.EditComment(ctx, commentID, req.Message); err != nil {
		return nil, err
	}

	// Update local copy to reflect changes for the response
	comment.Message = req.Message
	comment.IsEdited = true
	comment.UpdatedAt = time.Now()

	usersMap := s.hydrateUserIDs(ctx, []primitive.ObjectID{requesterID})
	return toCommentResponse(comment, usersMap), nil
}

// DeleteComment removes a comment. Only the original author may delete.
func (s *taskService) DeleteComment(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID, commentID primitive.ObjectID) error {
	// Verify task exists and requester has access
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTaskNotFound
		}
		return err
	}
	if !canAccess(task, requesterID) {
		return ErrUnauthorized
	}

	// Fetch the comment to check ownership
	comment, err := s.commentRepo.FindByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, repository.ErrCommentNotFound) {
			return ErrCommentNotFound
		}
		return err
	}

	// Only the original author may delete
	if comment.UserID != requesterID {
		return ErrCommentForbidden
	}

	// Ensure this comment belongs to the task in the URL
	if comment.TaskID != taskID {
		return ErrCommentNotFound
	}

	return s.commentRepo.DeleteComment(ctx, commentID)
}

// GetTaskComments returns a unified, time-ordered timeline of system activity
// logs (type "system") and user comments (type "comment") for a task.
// Only the assignee or assigned_by may view.
func (s *taskService) GetTaskComments(ctx context.Context, requesterID primitive.ObjectID, taskID primitive.ObjectID) ([]dto.TimelineItem, error) {
	// Verify task exists and requester has access
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

	// Fetch activities and comments concurrently (sequential for simplicity)
	activities, err := s.activityRepo.FindByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	comments, err := s.commentRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Collect all unique user IDs from both sources for a single batch fetch
	var allUserIDs []primitive.ObjectID
	for _, a := range activities {
		allUserIDs = append(allUserIDs, a.UserID)
	}
	for _, c := range comments {
		allUserIDs = append(allUserIDs, c.UserID)
	}
	usersMap := s.hydrateUserIDs(ctx, allUserIDs)

	// Build the unified timeline
	timeline := make([]dto.TimelineItem, 0, len(activities)+len(comments))

	for _, act := range activities {
		resp := dto.ActivityResponse{
			ID:        act.ID,
			TaskID:    act.TaskID,
			Message:   act.Message,
			CreatedAt: act.CreatedAt,
		}
		if u, ok := usersMap[act.UserID]; ok {
			resp.User = toUserSummary(u)
		}
		timeline = append(timeline, dto.TimelineItem{
			Type:      "system",
			CreatedAt: act.CreatedAt,
			Activity:  &resp,
		})
	}

	for i := range comments {
		c := &comments[i]
		timeline = append(timeline, dto.TimelineItem{
			Type:      "comment",
			CreatedAt: c.CreatedAt,
			Comment:   toCommentResponse(c, usersMap),
		})
	}

	// Sort the unified slice newest → oldest (most recent first)
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].CreatedAt.After(timeline[j].CreatedAt)
	})

	return timeline, nil
}

// ─── Mapping helpers ─────────────────────────────────────────────────────────

// toCommentResponse converts a models.Comment + user map into a dto.CommentResponse.
func toCommentResponse(c *models.Comment, usersMap map[primitive.ObjectID]*models.User) *dto.CommentResponse {
	resp := &dto.CommentResponse{
		ID:        c.ID,
		TaskID:    c.TaskID,
		Message:   c.Message,
		IsEdited:  c.IsEdited,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if u, ok := usersMap[c.UserID]; ok {
		resp.User = toUserSummary(u)
	}
	return resp
}


var _ TaskService = (*taskService)(nil)
