package service

import (
	"context"
	"fmt"
	"log"
	"task-management-system/repository"
	"task-management-system/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Notification Windows ─────────────────────────────────────────────────────

// notificationWindows defines the three reminder milestones.
// Each window covers a 24-hour band so the hourly ticker is guaranteed to hit it.
//
//	Key      — stored in Task.NotificationsSent to prevent re-sending
//	DaysOut  — how many days before due_date the reminder fires
var notificationWindows = []struct {
	Key     string
	DaysOut int
}{
	{Key: "7d", DaysOut: 7},
	{Key: "3d", DaysOut: 3},
	{Key: "1d", DaysOut: 1},
	{Key: "0d", DaysOut: 0},
}

// ─── Service ─────────────────────────────────────────────────────────────────

// EmailNotificationService is a background goroutine that periodically scans for
// tasks approaching their due date and dispatches reminder emails to the assignee
// (To) and the task creator/assigned_by (CC).
type EmailNotificationService struct {
	taskRepo repository.TaskRepository
	userRepo repository.UserRepository
	// interval controls how often the job wakes up (default: 1 hour).
	interval time.Duration
}

// NewEmailNotificationService constructs an EmailNotificationService with a
// default run interval of 1 hour.
func NewEmailNotificationService(taskRepo repository.TaskRepository, userRepo repository.UserRepository) *EmailNotificationService {
	return &EmailNotificationService{
		taskRepo: taskRepo,
		userRepo: userRepo,
		// interval: 1 * time.Hour,
		interval: 10 * time.Second,
	}
}

// Start launches the background goroutine. It respects ctx cancellation so
// the caller can shut it down cleanly.
func (s *EmailNotificationService) Start(ctx context.Context) {
	go func() {
		log.Println("[email-notif] background job started (interval:", s.interval, ")")
		// Run once immediately on startup, then on every tick.
		s.runOnce(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[email-notif] background job stopped")
				return
			case <-ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

// runOnce iterates over all three notification windows in one pass.
func (s *EmailNotificationService) runOnce(ctx context.Context) {
	now := time.Now()

	for _, w := range notificationWindows {
		// Query window: tasks whose due_date is between (now + daysOut - 1 day)
		// and (now + daysOut), i.e. a 24-hour band around the milestone.
		from := now.Add(time.Duration(w.DaysOut-1) * 24 * time.Hour)
		to := now.Add(time.Duration(w.DaysOut) * 24 * time.Hour)

		tasks, err := s.taskRepo.FindTasksDueForNotification(ctx, from, to, w.Key)
		if err != nil {
			log.Printf("[email-notif] error querying tasks (window=%s): %v\n", w.Key, err)
			continue
		}

		for _, task := range tasks {
			if err := s.sendReminder(ctx, task.ID.Hex(), w.Key, w.DaysOut); err != nil {
				log.Printf("[email-notif] failed to process task %s (window=%s): %v\n",
					task.ID.Hex(), w.Key, err)
			}
		}
	}
}

// sendReminder fetches full task + user data, builds the email, sends it, and
// marks the notification as sent so it is never repeated.
func (s *EmailNotificationService) sendReminder(ctx context.Context, taskIDHex, notifKey string, daysOut int) error {
	// Re-fetch task to get the latest data
	taskOID, err := primitive.ObjectIDFromHex(taskIDHex)
	if err != nil {
		return fmt.Errorf("invalid task id %s: %w", taskIDHex, err)
	}

	task, err := s.taskRepo.FindByID(ctx, taskOID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Fetch assignee
	assignee, err := s.userRepo.FindByID(ctx, task.Assignee)
	if err != nil {
		return fmt.Errorf("assignee not found for task %s: %w", taskIDHex, err)
	}

	// Fetch assigned_by (task creator) for CC
	assignedBy, err := s.userRepo.FindByID(ctx, task.AssignedBy)
	if err != nil {
		return fmt.Errorf("assigned_by not found for task %s: %w", taskIDHex, err)
	}

	// Build CC list — only include assigned_by if they have a different email
	var cc []string
	if assignedBy.Email != assignee.Email {
		cc = []string{assignedBy.Email}
	}

	subject := fmt.Sprintf(
		"[Task Reminder] \"%s\" is due in %d Day(s)",
		task.Title, daysOut,
	)
	loc, _ := time.LoadLocation("Asia/Kolkata")
	// task.DueDate.
    // In(loc).
    // Format("02 Jan 2006, 03:04 PM MST")
	htmlBody := utils.BuildReminderEmailBody(
		assignee.UserName,
		task.Title,
		task.Summary,
		task.DueDate.
    In(loc).
    Format("02 Jan 2006"),
		fmt.Sprintf("%d", daysOut),
	)

	if err := utils.SendEmail(assignee.Email, cc, subject, htmlBody); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Mark the notification as sent — prevents duplicate emails on next tick
	if err := s.taskRepo.MarkNotificationSent(ctx, task.ID, notifKey); err != nil {
		log.Printf("[email-notif] WARNING: email sent but failed to mark notification (%s/%s): %v\n",
			taskIDHex, notifKey, err)
	}

	log.Printf("[email-notif] reminder sent → task=%s window=%s to=%s cc=%v\n",
		taskIDHex, notifKey, assignee.Email, cc)
	return nil
}
