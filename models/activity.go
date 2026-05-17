package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Activity represents an audit log entry for changes made to a task.
type Activity struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID    primitive.ObjectID `bson:"task_id" json:"task_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"` // User who performed the action
	Message   string             `bson:"message" json:"message"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
