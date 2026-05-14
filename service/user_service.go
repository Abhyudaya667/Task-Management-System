package service

import (
	"context"
	"task-management-system/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	SearchEmails(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error)
}

type userService struct{
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService{
	return &userService{userRepo : userRepo}
}

func (s *userService) SearchEmails(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error){
	emails ,err := s.userRepo.FilterByEmail(ctx,text)
	if err!=nil {
		return nil,err
	}
	return emails,err
}

