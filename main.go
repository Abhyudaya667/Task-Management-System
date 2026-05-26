package main

import (
	"context"
	"os"
	"strings"
	"task-management-system/config"
	controllers "task-management-system/controller"
	"task-management-system/middleware"
	"task-management-system/repository"
	"task-management-system/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// ── MongoDB connection ────────────────────────────────────────────────────
	// config.ConnectDB() connects to MongoDB and stores the *mongo.Database
	// in config.DB. No AutoMigrate — MongoDB is schema-less.
	config.ConnectDB()

	// ── Dependency injection ──────────────────────────────────────────────────
	taskRepo     := repository.NewTaskRepository(config.DB)
	userRepo     := repository.NewUserRepository(config.DB)
	labelRepo    := repository.NewLabelRepository(config.DB)
	activityRepo := repository.NewActivityRepository(config.DB)
	commentRepo  := repository.NewCommentRepository(config.DB)
	taskSvc := service.NewTaskService(taskRepo, userRepo, activityRepo, commentRepo)
	userSvc := service.NewUserService(userRepo, labelRepo)

	// ── Background services ───────────────────────────────────────────────────
	escalationSvc := service.NewEscalationService(taskRepo, activityRepo)
	escalationSvc.Start(context.Background())

	emailNotifSvc := service.NewEmailNotificationService(taskRepo, userRepo)
	emailNotifSvc.Start(context.Background())

	taskCtrl := controllers.NewTaskController(taskSvc)
	userCtrl := controllers.NewUserController(userSvc)
	// ── Router ────────────────────────────────────────────────────────────────
	r := gin.Default()

	// ── CORS Middleware ───────────────────────────────────────────────────────
	allowedOriginsStr := os.Getenv("CORS_ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsStr != "" {
		allowedOrigins = strings.Split(allowedOriginsStr, ",")
		for i, o := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(o)
		}
	} else {
		allowedOrigins = []string{"*"}
	}

	configCors := cors.DefaultConfig()
	configCors.AllowOrigins = allowedOrigins
	configCors.AllowCredentials = true
	configCors.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(configCors))

	// 🔹 Auth routes (public)
	r.POST("/auth/register", controllers.Register)
	r.GET("/auth/verify-email", controllers.VerifyEmail)
	r.POST("/auth/login", controllers.Login)

	// 🔹 Protected auth routes
	r.POST("/auth/logout", middleware.AuthMiddleware(), controllers.Logout)
	r.GET("/auth/me",middleware.AuthMiddleware(),userCtrl.GetMyDetails)
	r.POST("/auth/refresh", controllers.RefreshAccessToken)

	taskRoutes := r.Group("/tasks")
	taskRoutes.Use(middleware.AuthMiddleware())
	{
		taskRoutes.POST("/", taskCtrl.CreateTask)
		taskRoutes.GET("/", taskCtrl.ListTasks)
		taskRoutes.GET("/:id", taskCtrl.GetTask)
		taskRoutes.PATCH("/:id", taskCtrl.UpdateTask)
		taskRoutes.DELETE("/:id", taskCtrl.DeleteTask)
		taskRoutes.GET("/:id/activities", taskCtrl.GetTaskActivities)

		// Comment routes
		taskRoutes.POST("/:id/comments", taskCtrl.AddComment)
		taskRoutes.GET("/:id/comments", taskCtrl.GetTaskComments)
		taskRoutes.PATCH("/:id/comments/:commentId", taskCtrl.EditComment)
		taskRoutes.DELETE("/:id/comments/:commentId", taskCtrl.DeleteComment)
	}
	r.GET("/users/search", middleware.AuthMiddleware(), userCtrl.SearchUserNamesByText)
	labelRoutes := r.Group("/labels")
	labelRoutes.Use(middleware.AuthMiddleware())
	{
		labelRoutes.GET("/", userCtrl.SearchLabels)
		labelRoutes.POST("/", userCtrl.AddNewLabel)
	}
	r.Run(":8080")
}
