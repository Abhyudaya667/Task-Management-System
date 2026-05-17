package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActivityResponse is the API response shape for a single activity log entry.
type ActivityResponse struct {
	ID        primitive.ObjectID `json:"id"`
	TaskID    primitive.ObjectID `json:"task_id"`
	User      *UserSummary       `json:"user"` // The user who performed the action
	Message   string             `json:"message"`
	CreatedAt time.Time          `json:"created_at"`
}
