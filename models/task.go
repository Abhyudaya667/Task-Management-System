package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Enums
type TaskType string
type TaskStatus string
type TaskPriority string

const (
	TaskTypeTask    TaskType = "task"
	TaskTypeBug     TaskType = "bug"
	TaskTypeFeature TaskType = "feature"

	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in-progress"
	StatusDone       TaskStatus = "done"

	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

// Task struct
type Task struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title" json:"title"`
	Summary     string             `bson:"summary" json:"summary"`
	Description string             `bson:"description" json:"description"`

	Type     TaskType   `bson:"type" json:"type"`
	Labels   []string   `bson:"labels" json:"labels"`
	Status   TaskStatus `bson:"status" json:"status"`
	Priority TaskPriority `bson:"priority" json:"priority"`

	Assignee   primitive.ObjectID `bson:"assignee" json:"assignee"` // single user
	AssignedBy primitive.ObjectID `bson:"assigned_by" json:"assigned_by"`

	StartDate time.Time `bson:"start_date" json:"start_date"`
	DueDate   time.Time `bson:"due_date" json:"due_date"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`

	IsDeleted bool `bson:"is_deleted,omitempty" json:"is_deleted,omitempty"`
}