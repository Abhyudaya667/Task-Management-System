package repository

import (
	"context"
	"errors"
	"log"
	"task-management-system/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	FindByIDs(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.User, error)
	FindByUserName(ctx context.Context, username string) (*models.User, error)
	FilterByUserName(ctx context.Context, text string) ([]string, error)
}

type userRepository struct {
	col *mongo.Collection
}
func createIndexes(col *mongo.Collection) error {
	ctx := context.Background()

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "username", Value: 1},
		},
		Options: options.Index().
			SetUnique(true).
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}),
	}

	_, err := col.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		log.Println("failed to create username index:", err)
		return err
	}
	return nil
}

func NewUserRepository(db *mongo.Database) UserRepository {
	col := db.Collection("users")

	createIndexes(col)

	return &userRepository{
		col: col,
	}
}
func (r *userRepository) FilterByUserName(ctx context.Context,text string) ([]string,error){
	filter := bson.M{
		"username": bson.M{
			"$regex":   "^" + text,
			"$options": "i",
		},
	}

	opts := options.Find().
		SetLimit(5).
		SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2,
		})


	cursor, err := r.col.Find(ctx, filter,opts)
	if err != nil{
		return nil,err
	}
	usernames:=[]string{}
	for cursor.Next(ctx) {
		var u models.User
		if err := cursor.Decode(&u); err != nil{
			return nil,err
		}
		usernames = append(usernames, u.UserName)
	}
	return usernames,cursor.Err()
}
func (r *userRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// FindByEmail looks up a single user by their email address.
// Used during task create/update to resolve assignee_email → ObjectID.
func (r *userRepository) FindByUserName(ctx context.Context, username string) (*models.User, error) {
	var user models.User

	opts := options.FindOne().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2,
	})

	err := r.col.FindOne(
		ctx,
		bson.M{"username": username},
		opts,
	).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

// FindByIDs fetches multiple users in a single round-trip and returns them
// as a map keyed by ObjectID — used to hydrate Assignee/AssignedBy in responses.
func (r *userRepository) FindByIDs(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.User, error) {
	cursor, err := r.col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[primitive.ObjectID]*models.User, len(ids))
	for cursor.Next(ctx) {
		var u models.User
		if err := cursor.Decode(&u); err != nil {
			return nil, err
		}
		u2 := u // avoid loop-var aliasing
		result[u2.ID] = &u2
	}
	return result, cursor.Err()
}