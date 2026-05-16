package repository

import (
	"context"
	"errors"
	"task-management-system/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrLabelExists = errors.New("label already exists")

type LabelRepository interface {
	CreateLabel(ctx context.Context, createdBy primitive.ObjectID, label string) (*models.Label, error)
	SearchLabels(ctx context.Context, text string, limit int) ([]models.Label, error)
}

type labelRepository struct {
	col *mongo.Collection
}

func NewLabelRepository(db *mongo.Database) LabelRepository {
	return &labelRepository{col: db.Collection("labels")}
}

func (r *labelRepository) CreateLabel(ctx context.Context, createdBy primitive.ObjectID, label string) (*models.Label, error) {
	var existing models.Label
	filter := bson.M{"label": label}

	err := r.col.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		return nil, ErrLabelExists
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}
	newLabel := models.Label{
		ID:        primitive.NewObjectID(),
		Name:      label,     
		CreatedBy: createdBy, 
	}

	_, err = r.col.InsertOne(ctx, newLabel)
	if err != nil {
		return nil, err
	}

	return &newLabel, nil
}

func (r *labelRepository) SearchLabels(ctx context.Context, text string, limit int) ([]models.Label, error) {
	filter := bson.M{
	"label": bson.M{
		"$regex":   "^" + text,
		"$options": "i",
	},
}

	opts := options.Find().SetLimit(int64(limit))
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var labels []models.Label
	if err := cursor.All(ctx, &labels); err != nil {
		return nil, err
	}

	return labels, nil
}