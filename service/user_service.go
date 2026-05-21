package service

import (
	"context"
	"task-management-system/dto"
	"task-management-system/models"
	"task-management-system/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	SearchUserNames(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error)
	AddLabel(ctx context.Context, requesterID primitive.ObjectID, label string) (*models.Label, error)
	SearchLabels(ctx context.Context, text string) ([]models.Label, error)
	FindDetailsByID(ctx context.Context,requsterid primitive.ObjectID)(*dto.UserSummary,error)
}

type userService struct{
	userRepo repository.UserRepository
	labelRepo repository.LabelRepository
}
func toUserSummary(u *models.User) *dto.UserSummary {
	if u == nil {
		return nil
	}
	return &dto.UserSummary{ID: u.ID, Name: u.UserName, Email: u.Email}
}
func NewUserService(userRepo repository.UserRepository, labelRepo repository.LabelRepository) UserService {
	return &userService{userRepo: userRepo, labelRepo: labelRepo}
}
func (s *userService) SearchUserNames(ctx context.Context,requesterid primitive.ObjectID,text string)([]string,error){
	usernames ,err := s.userRepo.FilterByUserName(ctx,text)
	if err!=nil {
		return nil,err
	}
	return usernames,err
}
func (s *userService) FindDetailsByID(ctx context.Context,requsterid primitive.ObjectID)(*dto.UserSummary,error){
	userdetail,err := s.userRepo.FindByID(ctx,requsterid)
	if err!=nil{
		return nil,err
	}
	return toUserSummary(userdetail),nil

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

