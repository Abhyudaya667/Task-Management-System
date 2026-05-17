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
	taskSvc := service.NewTaskService(taskRepo, userRepo, activityRepo)
	userSvc := service.NewUserService(userRepo, labelRepo)

	// ── Background services ───────────────────────────────────────────────────
	escalationSvc := service.NewEscalationService(taskRepo, activityRepo)
	escalationSvc.Start(context.Background())

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
	r.POST("/auth/login", controllers.Login)

	// 🔹 Protected auth routes
	r.POST("/auth/logout", middleware.AuthMiddleware(), controllers.Logout)
	r.POST("/auth/refresh", controllers.RefreshAccessToken)

	// ── Protected task routes ─────────────────────────────────────────────────
	//
	//  POST   /tasks/      → create a task  (caller = assigned_by)
	//  GET    /tasks/      → list tasks     (paginated, filters: status/priority/type/search)
	//  GET    /tasks/:id   → task detail    (assignee or assigned_by only)
	//  PATCH  /tasks/:id   → partial update (assigned_by only)
	//  DELETE /tasks/:id   → soft delete    (assigned_by only)
	//
	taskRoutes := r.Group("/tasks")
	taskRoutes.Use(middleware.AuthMiddleware())
	{
		taskRoutes.POST("/", taskCtrl.CreateTask)
		taskRoutes.GET("/", taskCtrl.ListTasks)
		taskRoutes.GET("/:id", taskCtrl.GetTask)
		taskRoutes.PATCH("/:id", taskCtrl.UpdateTask)
		taskRoutes.DELETE("/:id", taskCtrl.DeleteTask)
		taskRoutes.GET("/:id/activities", taskCtrl.GetTaskActivities)
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
