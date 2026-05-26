package repository

import (
	"context"
	"errors"
	"task-management-system/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrCommentNotFound is returned when a comment ID does not exist in the collection.
var ErrCommentNotFound = errors.New("comment not found")

// CommentInterface defines the persistence operations for user comments.
type CommentInterface interface {
	CreateComment(ctx context.Context, comment *models.Comment) error
	EditComment(ctx context.Context, id primitive.ObjectID, message string) error
	DeleteComment(ctx context.Context, id primitive.ObjectID) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.Comment, error)
	GetByTaskID(ctx context.Context, taskID primitive.ObjectID) ([]models.Comment, error)
}

type CommentRepository struct {
	col *mongo.Collection
}

func NewCommentRepository(db *mongo.Database) CommentInterface {
	return &CommentRepository{col: db.Collection("comments")}
}

// CreateComment inserts a new comment with auto-generated ID and timestamps.
func (c *CommentRepository) CreateComment(ctx context.Context, comment *models.Comment) error {
	comment.ID = primitive.NewObjectID()
	now := time.Now()
	comment.CreatedAt = now
	comment.UpdatedAt = now
	_, err := c.col.InsertOne(ctx, comment)
	return err
}

// EditComment updates the message of an existing comment and marks it as edited.
func (c *CommentRepository) EditComment(ctx context.Context, id primitive.ObjectID, message string) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{
		"message":    message,
		"is_edited":  true,
		"updated_at": time.Now(),
	}}
	result, err := c.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrCommentNotFound
	}
	return nil
}

// DeleteComment hard-deletes a comment by its ID.
func (c *CommentRepository) DeleteComment(ctx context.Context, id primitive.ObjectID) error {
	result, err := c.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrCommentNotFound
	}
	return nil
}

// FindByID fetches a single comment by its ID. Used for ownership checks.
func (c *CommentRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Comment, error) {
	var comment models.Comment
	err := c.col.FindOne(ctx, bson.M{"_id": id}).Decode(&comment)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}
	return &comment, nil
}

// GetByTaskID returns all comments for a task, ordered oldest-first.
func (c *CommentRepository) GetByTaskID(ctx context.Context, taskID primitive.ObjectID) ([]models.Comment, error) {
	filter := bson.M{"task_id": taskID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := c.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var comments []models.Comment
	if err := cursor.All(ctx, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}
