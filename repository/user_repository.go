// package repository

// import (
// 	"context"
// 	"errors"
// 	"task-management-system/models"

// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// 	"go.mongodb.org/mongo-driver/mongo"
// )

// var ErrUserNotFound = errors.New("user not found")

// type UserRepository interface {
// 	FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
// 	FindByIDs(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.User, error)
// }

// type userRepository struct {
// 	col *mongo.Collection
// }

// func NewUserRepository(db *mongo.Database) UserRepository {
// 	return &userRepository{col: db.Collection("users")}
// }

// func (r *userRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
// 	var user models.User
// 	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
// 	if errors.Is(err, mongo.ErrNoDocuments) {
// 		return nil, ErrUserNotFound
// 	}
// 	return &user, err
// }

// // FindByIDs fetches multiple users in a single round-trip and returns them
// // as a map keyed by ObjectID — used to hydrate Assignee/AssignedBy in task responses.
// func (r *userRepository) FindByIDs(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.User, error) {
// 	cursor, err := r.col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cursor.Close(ctx)

//		result := make(map[primitive.ObjectID]*models.User, len(ids))
//		for cursor.Next(ctx) {
//			var u models.User
//			if err := cursor.Decode(&u); err != nil {
//				return nil, err
//			}
//			u2 := u // avoid loop-var aliasing
//			result[u2.ID] = &u2
//		}
//		return result, cursor.Err()
//	}
package repository

import (
	"context"
	"errors"
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
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FilterByEmail(ctx context.Context, text string)([]string,error)
}

type userRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) UserRepository {
	return &userRepository{col: db.Collection("users")}
}
func (r *userRepository) FilterByEmail(ctx context.Context,text string) ([]string,error){
	filter := bson.M{
	"email": bson.M{
		"$regex": text,
		"$options": "i", 
	},
	}
	opts := options.Find().SetLimit(5)

	cursor, err := r.col.Find(ctx, filter,opts)
	if err != nil{
		return nil,err
	}
	emails := []string{}
	for cursor.Next(ctx) {
		var u models.User
		if err := cursor.Decode(&u); err != nil{
			return nil,err
		}
		emails = append(emails, u.Email)
	}
	return emails,cursor.Err()
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
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&user)
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