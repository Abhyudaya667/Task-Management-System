// package main

// import (
// 	"task-management-system/config"
// 	controllers "task-management-system/controller"

// 	"github.com/gin-gonic/gin"
// 	"task-management-system/middleware"
// )
// func main() {
// 	config.ConnectDB()

// 	r := gin.Default()

// 	// 🔹 Auth routes (public)
// 	r.POST("/auth/register", controllers.Register)
// 	r.POST("/auth/login", controllers.Login)

// 	// 🔹 Logout route (protected)
// 	r.POST("/auth/logout", middleware.AuthMiddleware(), controllers.Logout)

// 	// 🔹 Optional: refresh route (no middleware, uses refresh cookie)
// 	r.POST("/auth/refresh", controllers.RefreshAccessToken)

// 	// 🔹 Protected task routes
// 	taskRoutes := r.Group("/tasks")
// 	taskRoutes.Use(middleware.AuthMiddleware())
// 	{
// 		taskRoutes.GET("/", func(c *gin.Context) {
// 			userID, _ := c.Get("user_id")

// 			c.JSON(200, gin.H{
// 				"message": "Authorized",
// 				"user_id": userID,
// 			})
// 		})
// 	}

// 	r.Run(":8080")
// }
package main

import (
	"task-management-system/config"
	controllers "task-management-system/controller"
	"task-management-system/middleware"
	"task-management-system/repository"
	"task-management-system/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// ── MongoDB connection ────────────────────────────────────────────────────
	// config.ConnectDB() connects to MongoDB and stores the *mongo.Database
	// in config.DB. No AutoMigrate — MongoDB is schema-less.
	config.ConnectDB()

	// ── Dependency injection ──────────────────────────────────────────────────
	taskRepo := repository.NewTaskRepository(config.DB)
	userRepo := repository.NewUserRepository(config.DB)
	taskSvc  := service.NewTaskService(taskRepo, userRepo)
	taskCtrl := controllers.NewTaskController(taskSvc)

	// ── Router ────────────────────────────────────────────────────────────────
	r := gin.Default()

	// 🔹 Auth routes (public)
	r.POST("/auth/register", controllers.Register)
	r.POST("/auth/login", controllers.Login)

	// 🔹 Protected auth routes
	r.POST("/auth/logout",  middleware.AuthMiddleware(), controllers.Logout)
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
		taskRoutes.POST("/",    taskCtrl.CreateTask)
		taskRoutes.GET("/",     taskCtrl.ListTasks)
		taskRoutes.GET("/:id",  taskCtrl.GetTask)
		taskRoutes.PATCH("/:id", taskCtrl.UpdateTask)
		taskRoutes.DELETE("/:id", taskCtrl.DeleteTask)
	}

	r.Run(":8080")
}
