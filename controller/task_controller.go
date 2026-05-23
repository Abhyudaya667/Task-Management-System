package controllers

import (
	"errors"
	"net/http"
	"task-management-system/dto"
	"task-management-system/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaskController struct {
	svc service.TaskService
}

func NewTaskController(svc service.TaskService) *TaskController {
	return &TaskController{svc: svc}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// getRequesterID extracts the authenticated user's ObjectID from the Gin context.
// AuthMiddleware stores "user_id" as a primitive.ObjectID after converting
// the string from the JWT claim.
func getRequesterID(c *gin.Context) (primitive.ObjectID, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		return primitive.NilObjectID, false
	}
	id, ok := val.(primitive.ObjectID)
	return id, ok
}

// parseTaskID parses the ":id" route param as a MongoDB ObjectID hex string.
func parseTaskID(c *gin.Context) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(c.Param("id"))
}

// handleTaskError maps service sentinel errors to the correct HTTP status codes.
func handleTaskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
	case errors.Is(err, service.ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidID):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id format"})
	case errors.Is(err, service.ErrAssigneeNotFound):
		// Return 422 Unprocessable Entity — the request was valid JSON but the
		// referenced assignee email does not match any registered user.
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrDateInPast):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidDateRange):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// ─── POST /tasks/ ────────────────────────────────────────────────────────────

// CreateTask creates a new task.
// The authenticated user becomes assigned_by automatically.
// assignee_email in the body is resolved to an ObjectID by the service layer.
//
// Request body: dto.CreateTaskRequest  (see dto/task_dto.go)
// Response 201: { "data": dto.TaskDetail }
func (tc *TaskController) CreateTask(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := tc.svc.CreateTask(c.Request.Context(), requesterID, req)
	if err != nil {
		handleTaskError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": task})
}

// ─── GET /tasks/ ─────────────────────────────────────────────────────────────

// ListTasks returns paginated task summaries for the authenticated user.
// A task is visible when the caller is either the assignee or assigned_by.
//
// Query params — all optional:
//
//	page       int     default 1
//	page_size  int     default 10, max 100
//	status     string  pending | in-progress | done
//	priority   string  low | medium | high
//	type       string  task | bug | feature
//	search     string  case-insensitive substring match on title
//
// Response 200: { "data": dto.PaginatedTasksResponse }
func (tc *TaskController) ListTasks(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var params dto.TaskFilterParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := tc.svc.ListTasks(c.Request.Context(), requesterID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ─── GET /tasks/:id ──────────────────────────────────────────────────────────

// GetTask returns the full task detail including Description.
// Only the assignee or assigned_by may view; others receive 403.
//
// Response 200: { "data": dto.TaskDetail }
func (tc *TaskController) GetTask(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	taskID, err := parseTaskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	task, err := tc.svc.GetTaskByID(c.Request.Context(), requesterID, taskID)
	if err != nil {
		handleTaskError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ─── PATCH /tasks/:id ────────────────────────────────────────────────────────

// UpdateTask applies a partial update to a task.
// Only assigned_by (the creator) may edit; assignees receive 403.
// All fields are optional — only provided fields are written to MongoDB.
// If assignee_email is provided, it is resolved to a new assignee ObjectID.
//
// Request body: dto.UpdateTaskRequest
// Response 200: { "data": dto.TaskDetail }
func (tc *TaskController) UpdateTask(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	taskID, err := parseTaskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := tc.svc.UpdateTask(c.Request.Context(), requesterID, taskID, req)
	if err != nil {
		handleTaskError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ─── DELETE /tasks/:id ───────────────────────────────────────────────────────

// DeleteTask soft-deletes a task (sets is_deleted = true).
// Only assigned_by (the creator) may delete; assignees receive 403.
//
// Response 200: { "message": "task deleted successfully" }
func (tc *TaskController) DeleteTask(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	taskID, err := parseTaskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	if err := tc.svc.DeleteTask(c.Request.Context(), requesterID, taskID); err != nil {
		handleTaskError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task deleted successfully"})
}

// ─── GET /tasks/:id/activities ───────────────────────────────────────────────

// GetTaskActivities returns the full activity history of a task.
// Only the assignee or assigned_by may view; others receive 403.
//
// Response 200: { "data": []dto.ActivityResponse }
func (tc *TaskController) GetTaskActivities(c *gin.Context) {
	requesterID, ok := getRequesterID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	taskID, err := parseTaskID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	activities, err := tc.svc.GetTaskActivities(c.Request.Context(), requesterID, taskID)
	if err != nil {
		handleTaskError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": activities})
}
