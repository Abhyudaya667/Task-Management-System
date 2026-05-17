package repository

import (
	"context"
	"task-management-system/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ActivityRepository defines the persistence operations for activity logs.
type ActivityRepository interface {
	Create(ctx context.Context, activity *models.Activity) error
	FindByTaskID(ctx context.Context, taskID primitive.ObjectID) ([]models.Activity, error)
}

type activityRepository struct {
	col *mongo.Collection
}

func NewActivityRepository(db *mongo.Database) ActivityRepository {
	return &activityRepository{col: db.Collection("activities")}
}

// Create inserts a new activity log entry.
func (r *activityRepository) Create(ctx context.Context, activity *models.Activity) error {
	activity.ID = primitive.NewObjectID()
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now()
	}
	_, err := r.col.InsertOne(ctx, activity)
	return err
}

// FindByTaskID returns all activities for a task, newest first.
func (r *activityRepository) FindByTaskID(ctx context.Context, taskID primitive.ObjectID) ([]models.Activity, error) {
	filter := bson.M{"task_id": taskID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var activities []models.Activity
	if err := cursor.All(ctx, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}
