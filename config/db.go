package config

import (
	"context"
	"log"
	"time"
	"os"
	"github.com/joho/godotenv"

	// "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Database

func ConnectDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		log.Fatal("MONGO_URI not found in environment")
	}
	clientOptions := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("MongoDB connection error:", err)
	}

	// Ping to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("MongoDB ping failed:", err)
	}

	DB = client.Database("task_manager")
	// collection := DB.Collection("users")

	// user := bson.M{
	// 	"name": "test",
	// 	"email": "test@gmail.com",
	// }

	// collection.InsertOne(context.Background(), user)
	log.Println(" Connected to MongoDB Atlas Cluster")
}