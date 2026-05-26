package repository

import (
	"context"
	"errors"
	"task-management-system/dto"
	"task-management-system/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrNotFound = errors.New("task not found")

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.Task, error)
	FindAll(ctx context.Context, userID primitive.ObjectID, params dto.TaskFilterParams) ([]models.Task, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, fields bson.M) error
	SoftDelete(ctx context.Context, id primitive.ObjectID) error
	// FindTasksDueForEscalation returns non-deleted, non-done tasks with the given
	// priority whose priority_set_at is older than olderThan.
	FindTasksDueForEscalation(ctx context.Context, priority models.TaskPriority, olderThan time.Time) ([]models.Task, error)
	// FindTasksDueForNotification returns active, non-done tasks whose due_date
	// falls within [from, to] and have not yet received the given notifKey.
	FindTasksDueForNotification(ctx context.Context, from, to time.Time, notifKey string) ([]models.Task, error)
	// MarkNotificationSent atomically appends notifKey to the task's
	// notifications_sent array so it is never re-sent.
	MarkNotificationSent(ctx context.Context, taskID primitive.ObjectID, notifKey string) error
	showAllComments(ctx context.Context,taskid primitive.ObjectID) ([]models.Comment,error)
}

type taskRepository struct {
	col *mongo.Collection
}

func NewTaskRepository(db *mongo.Database) TaskRepository {
	return &taskRepository{col: db.Collection("tasks")}
}

// ─── Create ──────────────────────────────────────────────────────────────────

func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	task.ID = primitive.NewObjectID()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.IsDeleted = false

	_, err := r.col.InsertOne(ctx, task)
	return err
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func (r *taskRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Task, error) {
	filter := bson.M{
		"_id":        id,
		// "is_deleted": bson.M{"$ne": true},
	}

	var task models.Task
	err := r.col.FindOne(ctx, filter).Decode(&task)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	return &task, err
}

// ─── FindAll ─────────────────────────────────────────────────────────────────

// FindAll returns paginated tasks where the user is the assignee OR assigned_by.
// Filters: status, priority, type, and a case-insensitive title search.
func (r *taskRepository) FindAll(
	ctx context.Context,
	userID primitive.ObjectID,
	params dto.TaskFilterParams,
) ([]models.Task, int64, error) {

	filter := bson.M{
	"$or": bson.A{
		bson.M{"assignee": userID},
		bson.M{"assigned_by": userID},
	},
	}

	if params.Deleted {
		filter["is_deleted"] = true
	} else {
		filter["is_deleted"] = bson.M{"$ne": true}
	}

	if params.Status != "" {
		filter["status"] = params.Status
	}
	if params.Priority != "" {
		filter["priority"] = params.Priority
	}
	if params.Type != "" {
		filter["type"] = params.Type
	}
	if params.Search != "" {
		// Case-insensitive prefix match on title
		filter["title"] = bson.M{"$regex": params.Search, "$options": "i"}
	}

	// Total count (for pagination metadata)
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	skip := int64((params.Page - 1) * params.PageSize)
	limit := int64(params.PageSize)

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update applies a bson.M patch to a non-deleted task.
// The service layer is responsible for building the $set map.
func (r *taskRepository) Update(ctx context.Context, id primitive.ObjectID, fields bson.M) error {
	filter := bson.M{
		"_id":        id,
		"is_deleted": bson.M{"$ne": true},
	}

	fields["updated_at"] = time.Now()

	res, err := r.col.UpdateOne(ctx, filter, bson.M{"$set": fields})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

func (r *taskRepository) SoftDelete(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{
		"_id":        id,
		"is_deleted": bson.M{"$ne": true},
	}

	res, err := r.col.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{
			"is_deleted": true,
			"updated_at": time.Now(),
		},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// ─── FindTasksDueForEscalation ────────────────────────────────────────────────

// FindTasksDueForEscalation returns all active tasks (non-deleted, not done)
// with the given priority whose priority_set_at is older than olderThan.
// The escalation background job calls this for each escalatable priority level.
func (r *taskRepository) FindTasksDueForEscalation(ctx context.Context, priority models.TaskPriority, olderThan time.Time) ([]models.Task, error) {
	filter := bson.M{
		"is_deleted":      bson.M{"$ne": true},
		"status":          bson.M{"$ne": string(models.StatusDone)},
		"priority":        string(priority),
		"priority_set_at": bson.M{"$lt": olderThan},
	}

	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ─── FindTasksDueForNotification ─────────────────────────────────────────────

// FindTasksDueForNotification returns all active (non-deleted, non-done) tasks
// whose due_date falls within the window [from, to] and that have not yet
// received the notification identified by notifKey ("7d", "3d", or "1d").
func (r *taskRepository) FindTasksDueForNotification(ctx context.Context, from, to time.Time, notifKey string) ([]models.Task, error) {
	filter := bson.M{
		"is_deleted": bson.M{"$ne": true},
		"status":     bson.M{"$ne": string(models.StatusDone)},
		"due_date":   bson.M{"$gte": from, "$lt": to},
		// Only return tasks that have NOT already been notified for this window
		"notifications_sent": bson.M{"$not": bson.M{"$elemMatch": bson.M{"$eq": notifKey}}},
	}

	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ─── MarkNotificationSent ─────────────────────────────────────────────────────

// MarkNotificationSent appends notifKey to the task's notifications_sent array
// using a MongoDB $addToSet so the key is never duplicated.
func (r *taskRepository) MarkNotificationSent(ctx context.Context, taskID primitive.ObjectID, notifKey string) error {
	filter := bson.M{"_id": taskID}
	update := bson.M{
		"$addToSet": bson.M{"notifications_sent": notifKey},
		"$set":      bson.M{"updated_at": time.Now()},
	}
	_, err := r.col.UpdateOne(ctx, filter, update)
	return err
}

func (r *taskRepository)showAllComments(ctx context.Context,taskid primitive.ObjectID) ([]models.Comment,error){
	return nil,nil
}
