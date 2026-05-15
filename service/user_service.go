package service

import (
	"context"
	"task-management-system/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"task-management-system/models"
)

type UserService interface {
	SearchEmails(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error)
	AddLabel(ctx context.Context, requesterID primitive.ObjectID, label string) (*models.Label, error)
	SearchLabels(ctx context.Context, text string) ([]models.Label, error)
}

type userService struct{
	userRepo repository.UserRepository
	labelRepo repository.LabelRepository
}

func NewUserService(userRepo repository.UserRepository, labelRepo repository.LabelRepository) UserService {
	return &userService{userRepo: userRepo, labelRepo: labelRepo}
}

func (s *userService) SearchEmails(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error){
	emails ,err := s.userRepo.FilterByEmail(ctx,text)
	if err!=nil {
		return nil,err
	}
	return emails,err
}

func (s *userService) AddLabel(ctx context.Context, requesterID primitive.ObjectID, label string) (*models.Label, error) {
	newLabel, err := s.labelRepo.CreateLabel(ctx, requesterID, label)
	if err != nil {
		return nil, err
	}
	return newLabel, nil
}

func (s *userService) SearchLabels(ctx context.Context, text string) ([]models.Label, error) {
	return s.labelRepo.SearchLabels(ctx, text, 5)
}

