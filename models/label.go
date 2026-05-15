package models

import "go.mongodb.org/mongo-driver/bson/primitive"


type Label struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"label"         json:"label"      binding:"required"`
	CreatedBy primitive.ObjectID `bson:"creator"       json:"created_by" binding:"required"`
}
