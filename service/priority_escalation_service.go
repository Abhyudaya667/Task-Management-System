package service

import (
	"context"
	"fmt"
	"log"
	"task-management-system/models"
	"task-management-system/repository"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ─── Escalation Rules ────────────────────────────────────────────────────────

// escalationRules defines how many days a task must sit at a priority level
// before it is automatically bumped to the next level.
// Only three levels can escalate; "critical" is the ceiling.
var escalationRules = []struct {
	From      models.TaskPriority
	To        models.TaskPriority
	AfterMinutes int
}{
	{From: models.PriorityLow, To: models.PriorityMedium, AfterMinutes: 1},
	{From: models.PriorityMedium, To: models.PriorityHigh, AfterMinutes: 2},
	{From: models.PriorityHigh, To: models.PriorityCritical, AfterMinutes: 3},
}

// ─── Service ─────────────────────────────────────────────────────────────────

// EscalationService runs a background goroutine that periodically auto-escalates
// task priorities based on how long they have been at their current level.
type EscalationService struct {
	taskRepo     repository.TaskRepository
	activityRepo repository.ActivityRepository
	// interval controls how often the job wakes up (default: 1 hour).
	interval time.Duration
}

// NewEscalationService constructs an EscalationService.
func NewEscalationService(taskRepo repository.TaskRepository, activityRepo repository.ActivityRepository) *EscalationService {
	return &EscalationService{
		taskRepo:     taskRepo,
		activityRepo: activityRepo,
		interval: 10 * time.Second,
	}
}

// Start launches the background goroutine. It respects ctx cancellation so
// the caller can stop it cleanly during shutdown.
func (s *EscalationService) Start(ctx context.Context) {
	go func() {
		log.Println("[escalation] background job started (interval:", s.interval, ")")
		// Run once immediately on startup, then on every tick.
		s.runOnce(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[escalation] background job stopped")
				return
			case <-ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

// runOnce processes all escalation rules in one pass.
func (s *EscalationService) runOnce(ctx context.Context) {
	for _, rule := range escalationRules {
		// threshold := time.Now().Add(-time.Duration(rule.AfterDays) * 24 * time.Hour)
		threshold := time.Now().Add(-time.Duration(rule.AfterMinutes) * time.Minute)

		tasks, err := s.taskRepo.FindTasksDueForEscalation(ctx, rule.From, threshold)
		if err != nil {
			log.Printf("[escalation] error querying tasks (priority=%s): %v\n", rule.From, err)
			continue
		}

		for _, task := range tasks {
			if err := s.escalate(ctx, task, rule.From, rule.To); err != nil {
				log.Printf("[escalation] failed to escalate task %s: %v\n", task.ID.Hex(), err)
			}
		}
	}
}

// escalate bumps a single task's priority and logs an activity.
func (s *EscalationService) escalate(ctx context.Context, task models.Task, from, to models.TaskPriority) error {
	now := time.Now()
	patch := bson.M{
		"priority":        string(to),
		"priority_set_at": now,
	}

	if err := s.taskRepo.Update(ctx, task.ID, patch); err != nil {
		return err
	}

	// Log activity using a zero UserID to indicate a system action.
	msg := fmt.Sprintf("Priority auto-escalated from '%s' to '%s'", from, to)
	_ = s.activityRepo.Create(ctx, &models.Activity{
		TaskID:  task.ID,
		Message: msg,
	})

	log.Printf("[escalation] task %s: %s → %s\n", task.ID.Hex(), from, to)
	return nil
}
