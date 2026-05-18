package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PendingRegistration holds a registration request that is waiting for email
// verification. Records are automatically cleaned up by MongoDB's TTL index
// after 24 hours, so they never pollute the users collection.
type PendingRegistration struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	UserName       string             `bson:"username"`
	Email          string             `bson:"email"`
	HashedPassword string             `bson:"hashed_password"`
	CreatedAt      time.Time          `bson:"created_at"`
	// TTL index on this field auto-deletes the document after 24 h.
	ExpiresAt time.Time `bson:"expires_at"`
}
