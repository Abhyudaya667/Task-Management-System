package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type CreateCommentRequest struct {
	Message string `json:"message" binding:"required,min=1,max=2000"`
}

type UpdateCommentRequest struct {
	Message string `json:"message" binding:"required,min=1,max=2000"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────


type CommentResponse struct {
	ID        primitive.ObjectID `json:"id"`
	TaskID    primitive.ObjectID `json:"task_id"`
	User      *UserSummary       `json:"user"`
	Message   string             `json:"message"`
	IsEdited  bool               `json:"is_edited"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}


type TimelineItem struct {
	Type      string            `json:"type"`               // "system" | "comment"
	CreatedAt time.Time         `json:"created_at"`         // canonical timestamp for sorting
	Activity  *ActivityResponse `json:"activity,omitempty"` // set when Type == "system"
	Comment   *CommentResponse  `json:"comment,omitempty"`  // set when Type == "comment"
}
