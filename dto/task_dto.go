package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Request DTOs ────────────────────────────────────────────────────────────

// CreateTaskRequest is what the frontend sends.
// assignee_email is a plain email string — the service resolves it to an ObjectID
// internally before writing to MongoDB. The caller never needs to know user IDs.
type CreateTaskRequest struct {
	Title         string    `json:"title"          binding:"required,min=3,max=200"`
	Summary       string    `json:"summary"        binding:"required,max=500"`
	Description   string    `json:"description"    binding:"omitempty,max=5000"`
	Type          string    `json:"type"           binding:"required,oneof=task bug feature"`
	Labels        []string  `json:"labels"`
	Status        string    `json:"status"         binding:"omitempty,oneof=todo in-progress in-review done"`
	Priority      string    `json:"priority"       binding:"required,oneof=low medium high critical"`
	AssigneeUserName string    `json:"assignee_username" binding:"required"`
	StartDate     time.Time `json:"start_date"     binding:"required"`
	DueDate       time.Time `json:"due_date"       binding:"required"`
}

// UpdateTaskRequest — all fields optional for partial updates (PATCH).
// assignee_email follows the same email-based convention as CreateTaskRequest.
type UpdateTaskRequest struct {
	Title         *string    `json:"title"          binding:"omitempty,min=3,max=200"`
	Summary       *string    `json:"summary"        binding:"omitempty,max=500"`
	Description   *string    `json:"description"    binding:"omitempty,max=5000"`
	Type          *string    `json:"type"           binding:"omitempty,oneof=task bug feature"`
	Labels        []string   `json:"labels"`
	Status        *string    `json:"status"         binding:"omitempty,oneof=todo in-progress in-review done"`
	Priority      *string    `json:"priority"       binding:"omitempty,oneof=low medium high critical"`
	AssigneeUserName string    `json:"assignee_username" binding:"required"`
	StartDate     *time.Time `json:"start_date"`
	DueDate       *time.Time `json:"due_date"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

// UserSummary is embedded in task responses so the frontend always has
// both id, name and email — never needs to do a separate user lookup.
type UserSummary struct {
	ID    primitive.ObjectID `json:"id"`
	Name  string             `json:"name"`
	Email string             `json:"email"`
}

// TaskSummary is returned by GET /tasks/ (list view — rectangular cards).
// Description is intentionally omitted; it is only included in TaskDetail.
type TaskSummary struct {
	ID       primitive.ObjectID `json:"id"`
	Title    string             `json:"title"`
	Summary  string             `json:"summary"`
	Type     string             `json:"type"`
	Labels   []string           `json:"labels"`
	Status   string             `json:"status"`
	Priority string             `json:"priority"`

	Assignee   *UserSummary `json:"assignee"`
	AssignedBy *UserSummary `json:"assigned_by"`

	StartDate time.Time `json:"start_date"`
	DueDate   time.Time `json:"due_date"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskDetail is returned by GET /tasks/:id and all mutation responses.
// Includes the full Description field.
type TaskDetail struct {
	ID          primitive.ObjectID `json:"id"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	Description string             `json:"description"`
	Type        string             `json:"type"`
	Labels      []string           `json:"labels"`
	Status      string             `json:"status"`
	Priority    string             `json:"priority"`

	Assignee   *UserSummary `json:"assignee"`
	AssignedBy *UserSummary `json:"assigned_by"`

	StartDate time.Time `json:"start_date"`
	DueDate   time.Time `json:"due_date"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── Pagination ──────────────────────────────────────────────────────────────

type PaginatedTasksResponse struct {
	Tasks      []TaskSummary `json:"tasks"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// ─── Query filter params ─────────────────────────────────────────────────────

type TaskFilterParams struct {
	Status   string `form:"status"`
	Priority string `form:"priority"`
	Type     string `form:"type"`
	Search   string `form:"search"` // case-insensitive title match
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=10"`
}
