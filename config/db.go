package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
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

	// ── TTL index on pending_registrations ────────────────────────────────────
	// Setting expireAfterSeconds=0 means MongoDB removes the document exactly at
	// the time stored in expires_at, so no manual cleanup is ever needed.
	pendingCol := DB.Collection("pending_registrations")
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	if _, err := pendingCol.Indexes().CreateOne(ctx, ttlIndex); err != nil {
		log.Println("[warning] could not create TTL index on pending_registrations:", err)
	}

	log.Println("Connected to MongoDB Atlas Cluster")
}