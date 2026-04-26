package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name" binding:"required"`
	Email     string             `bson:"email" json:"email" binding:"required,email"`
	Password  string             `bson:"password" json:"password" binding:"required,min=6"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
