package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)


type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID    primitive.ObjectID `bson:"task_id" json:"task_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"` // Author of the comment
	Message   string             `bson:"message" json:"message"`
	IsEdited  bool               `bson:"is_edited" json:"is_edited"` // True after any edit
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}